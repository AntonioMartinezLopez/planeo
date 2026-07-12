# Outbox Pattern Architecture Cleanup

## Purpose

The transactional outbox pattern for `services/email` (delivered in a prior spec/plan cycle) works functionally, but was verified with three architectural concerns that mix responsibilities across layers in a way that won't scale as the app grows:

1. `services/email/internal/infra/email/email_service.go` (an IMAP-fetching infra adapter) builds the Kafka event payload itself — it imports `libs/events` purely for a DTO type, pulling a transitive Kafka-client dependency into what should be a pure "fetch mail" concern.
2. `mail_repository.go`'s `SaveFetchedMails` combines two separate writes (a `mails` row and an `outbox` row) inside one opaque repository function, with the transaction boundary hidden entirely inside the repository. There's no reusable way for a *service* to orchestrate multiple, independent repository writes within one transaction.
3. The `outbox-relay` sidecar has an Air hot-reload config that's unlikely to ever be used (the sidecar isn't expected to run standalone with hot-reload locally).

This spec addresses all three. It does not add new functionality — it restructures existing, working code along cleaner boundaries.

## Non-goals

- No behavior change to the actual outbox semantics (claim/TTL, poison-row quarantine, ordering guarantees) — all of that is unchanged and already covered by existing tests.
- No retrofit of `setting_repository.go` or any of `services/core`'s repositories onto the new `WithTx`/`Querier` pattern — scoped to `services/email`'s mail+outbox repositories only. The utility is designed generically enough to be adopted elsewhere later, on an as-needed basis.
- No change to `outbox_repository.go`'s relay-facing methods (`FetchBatch`, `MarkProcessed`, `MarkFailed`) — they remain single-statement operations calling `c.db` directly; only the new `CreateOutboxEvent` method (used by the transactional write path) goes through the new `db.Q` accessor.
- No change to `services/core`'s subscribe side, or to `libs/events`'s public API — `PublishEmailReceived`/`SubscribeEmailReceived` keep their exact signatures.

## 1. `libs/events/contracts` extraction

`EmailCreatedPayload` and `EmailReceivedTopic` (currently defined in `libs/events/email_received.go`, alongside `EventService`'s Kafka pub/sub mechanics) move into a new package, `libs/events/contracts`, with zero external imports beyond the standard library. Keeping the topic name and its payload shape defined together in one small package means both sides of the contract (the topic string and the JSON shape) evolve as one unit rather than as two independently-hardcoded values.

`libs/events` imports `contracts` and continues to use it internally — `PublishEmailReceived`/`SubscribeEmailReceived` are otherwise unchanged, so `services/core`'s subscribe path is unaffected.

`domain/mail` imports `contracts` directly (never `libs/events`) to build the outbox payload — this is what keeps the domain layer free of any transitive Kafka-client (`twmb/franz-go`) dependency, matching the "zero infrastructure imports" rule CLAUDE.md states for the domain layer.

Forward-looking note (not part of this spec's scope, recorded for later): when schema-registry work is picked up, the codegen'd/versioned payload types would replace this same small `contracts` package — not get buried inside `libs/events`'s Kafka-mechanics file, and not duplicated per-service. Both the writing side (`domain/mail`) and the reading side (`libs/events`'s subscribe path) would continue to import this one place. This does not resolve the separate, still-open question of *where serialization logic* (e.g. embedding a schema ID) would live — only where the type definition lives.

## 2. Moving event construction into `domain/mail`

`domain/mail`'s input model changes. Instead of the caller (infra/email) pre-building `mail.FetchedMail{Mail, Event, UID}` (where `Event` is a fully-built `OutboxEvent` with topic/key/JSON payload), the caller now passes only raw fetched-mail data:

```go
// domain/mail/model.go
type RawFetchedMail struct {
    MessageID      string
    SettingID      int
    OrganizationID int
    Subject        string
    Sender         string
    Body           string
    Date           time.Time
    UID            uint32
}
```

`domain/mail.Service.SaveFetchedMails(ctx, raws []RawFetchedMail) ([]SaveResult, error)` internally builds, per raw mail, a `NewMail` and an `OutboxEvent` (topic `contracts.EmailReceivedTopic`, key `[]byte(strconv.Itoa(raw.OrganizationID))`, payload `json.Marshal(contracts.EmailCreatedPayload{...})`), then orchestrates the transactional write (see Section 4).

Consequence: `services/email/internal/infra/email/email_service.go` drops its `encoding/json` and `planeo/libs/events` imports entirely. It only imports `domain/mail`, mapping its own fetched `Email` structs into `RawFetchedMail` before calling `mailService.SaveFetchedMails`. This is the actual concern-separation requested: infra/email becomes purely "fetch mail from IMAP, hand raw data to the domain," with zero knowledge of what a Kafka event looks like.

## 3. `libs/db` WithTx/Querier utility + `services/email` Client migration

`libs/db` gains a `Querier` interface (satisfied structurally by both `*pgxpool.Pool` and `pgx.Tx` — no adapter needed) and two functions:

```go
type Querier interface {
    Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx) //nolint:errcheck

    txCtx := context.WithValue(ctx, txKey{}, tx)
    if err := fn(txCtx); err != nil {
        return err
    }
    return tx.Commit(ctx)
}

func Q(ctx context.Context, pool *pgxpool.Pool) Querier {
    if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
        return tx
    }
    return pool
}
```

`services/email/internal/infra/postgres/client.go`'s `NewClient` switches from calling `pgxpool.New(...)` directly to `db.InitializeDatabaseConnection(ctx, connString).DB`, mirroring `services/core`'s existing pattern exactly (confirmed identical in `services/core/internal/infra/postgres/client.go`). `Client`'s `db *pgxpool.Pool` field keeps its exact name and type, so `setting_repository.go` needs zero changes — only the construction path in `client.go` changes.

## 4. Mail+outbox repository split, orchestrated at the service level

The `Repository` port (`domain/mail/ports.go`) changes shape from one combined method to three, so the *service* — not the repository — decides transaction boundaries and orchestrates the two separate writes:

```go
type Repository interface {
    CreateMail(ctx context.Context, m NewMail) (mailID int, inserted bool, err error)
    CreateOutboxEvent(ctx context.Context, mailID int, event OutboxEvent) error
    WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
```

```go
// domain/mail/service.go
func (s *service) SaveFetchedMails(ctx context.Context, raws []RawFetchedMail) ([]SaveResult, error) {
    var results []SaveResult
    err := s.repository.WithTransaction(ctx, func(ctx context.Context) error {
        for _, raw := range raws {
            newMail, event := buildMailAndEvent(raw)
            mailID, inserted, err := s.repository.CreateMail(ctx, newMail)
            if err != nil {
                return err
            }
            if inserted {
                if err := s.repository.CreateOutboxEvent(ctx, mailID, event); err != nil {
                    return err
                }
            }
            results = append(results, SaveResult{UID: raw.UID, Inserted: inserted})
        }
        return nil
    })
    return results, err
}
```

```go
// infra/postgres/mail_repository.go
func (c *Client) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
    return db.WithTx(ctx, c.db, fn)
}

func (c *Client) CreateMail(ctx context.Context, m mail.NewMail) (int, bool, error) {
    q := db.Q(ctx, c.db)
    // ... same INSERT ... ON CONFLICT (setting_id, message_id) DO NOTHING ... RETURNING id, using q instead of tx
}
```

```go
// infra/postgres/outbox_repository.go
func (c *Client) CreateOutboxEvent(ctx context.Context, mailID int, event mail.OutboxEvent) error {
    q := db.Q(ctx, c.db)
    // ... same INSERT INTO outbox ..., using q instead of tx
}
```

This is the corrected hexagonal shape: the domain layer never imports a Postgres or pgx type — `WithTransaction`'s signature is a plain `func(ctx context.Context) error` callback, defined entirely in terms of the port's own vocabulary. The repository provides the transaction *mechanism* (via `libs/db.WithTx` internally) without hiding the orchestration of *which writes happen together* — that decision lives in the service, where business logic belongs.

`FetchBatch`/`MarkProcessed`/`MarkFailed` (the relay-facing side of `outbox_repository.go`, used by `libs/outbox.Store` — unrelated to this transactional write path) are unchanged: each remains a single atomic SQL statement calling `c.db` directly, with no need for `db.Q`.

## Testing implications

- `domain/mail/service_test.go` is restructured: `Repository`'s mock now has three methods instead of one. `WithTransaction`'s mock expectation uses mockery's `RunAndReturn` to actually invoke the passed callback (so the nested `CreateMail`/`CreateOutboxEvent` expectations fire within it), rather than a bare `Return(nil)`. Test cases: successful save (mail inserted, outbox event created), duplicate mail (mail not inserted, outbox event NOT created, no error), repository error at each of the three call points (propagates correctly, and does not call subsequent steps).
- `services/email/internal/test/mail/mail_test.go`'s integration test is updated to call the new repository methods directly — `env.DB.WithTransaction(ctx, func(ctx) error { ...call CreateMail, then CreateOutboxEvent if inserted... })` — matching how the existing test already exercises the repository layer directly (not through the domain service or its mocks). Same dedup/transactional-atomicity behavior under test (a duplicate mail's transaction is a safe no-op for both tables), just expressed against the new, split method shape instead of the old single `SaveFetchedMails` repository method.
- `services/email/internal/test/outbox/outbox_test.go` (Task 8's integration test) is unaffected — it tests `FetchBatch`/`MarkProcessed`/`MarkFailed`, none of which change.
- `libs/db` gets a new unit test for `WithTx`/`Q` — given these need a real Postgres connection to meaningfully test (a fake won't exercise real transaction semantics), this is an integration-style test using the existing testcontainer pattern, verifying: a transaction that returns an error from `fn` rolls back (no rows persisted), a transaction that succeeds commits, and `Q(ctx, pool)` returns the pool when no transaction is in the context vs. the tx when one is.

## Explicitly deferred / out of scope for this spec

- Retrofitting `setting_repository.go` or any `services/core` repository onto `WithTx`/`Q`.
- Routing `outbox_repository.go`'s `FetchBatch`/`MarkProcessed`/`MarkFailed` through `db.Q` for stylistic consistency — they don't need it.
- Schema-registry serialization logic and where it lives (writing service vs. shared library) — still an open question from the original outbox-pattern spec, unaffected by this cleanup.
