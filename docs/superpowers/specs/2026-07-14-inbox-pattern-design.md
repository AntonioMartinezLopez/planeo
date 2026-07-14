# Inbox Pattern for services/core's Kafka Consumer

## Purpose

`services/email`'s outbox pattern solved the producer-side dual-write problem (IMAP mark-seen vs. Kafka publish) and now durably guarantees at-least-once delivery of `email-received` events. `services/core`'s consumer side has the mirror-image problem: today, `libs/events.Subscribe` runs the entire business-logic callback (`CreateEmailReceivedCallback` — create a `Request`, call the LLM for field extraction and classification, update the `Request`) synchronously inline in the Kafka poll loop, committing the offset only if that callback succeeds. This has two problems:

1. **No idempotency.** If the callback partially succeeds (e.g. the `Request` is created, but the process crashes before the LLM classification step completes and the offset is committed), Kafka redelivers the same message on restart, and the callback runs again from scratch — creating a duplicate `Request` or repeating side effects, with no protection against it.
2. **Slow processing blocks acking.** LLM extraction and classification are slow, network-dependent calls. Because they run inline before the offset commits, a slow or flaky LLM call directly risks Kafka consumer-group timeouts (`max.poll.interval.ms`) and delays how quickly the consumer group's lag drains.

This spec introduces the **Inbox pattern** — a `libs/outbox`-style reusable engine (`libs/inbox`) plus a new dedicated binary, `services/core/cmd/email-received-consumer`, that:
- Durably persists each raw Kafka record into an `inbox` table before committing the offset (deduped on the Kafka coordinate), decoupling "safely acknowledged" from "fully processed."
- Processes persisted inbox records asynchronously, in insertion order, via an injected handler — mirroring the outbox relay's claim/retry/quarantine mechanics in reverse.

This fully replaces `services/core`'s current inline Kafka subscription. As a preliminary step, it also renames the existing `outbox-relay` sidecar to follow an explicit `<topic-name>-producer`/`<topic-name>-consumer` naming convention, adopted before any inbox code is written (see Section 1).

## Non-goals

- No functional change to `services/email`'s outbox pattern, its schema, or its relay logic — Section 1's changes are naming/identity only (folder, image, container, task, logger tag, env var names), not behavior.
- No redesign of Kafka consumer-group semantics beyond what's needed here. There are two independent scaling mechanisms in this design, only one of which involves a "consumer group":
  - **`Consumer` (Kafka-receiving side):** scales via Kafka's native consumer-group mechanism — multiple `Consumer` instances that share the same group name have the topic's partitions split between them by Kafka itself (`kgo.ConsumerGroup(groupName)`), which already works today with no redesign needed for the basic mechanism. What's still deferred is the deeper tuning nobody has designed yet: partition count relative to instance count, rebalancing strategy, behavior during a rebalance mid-scale-out, etc. — the "known simplification" flagged during the original NATS→Kafka migration.
  - **`Worker` (inbox-table-reading side):** scales via a completely separate, already-fully-designed mechanism — the same `FOR UPDATE SKIP LOCKED` atomic claim `outbox.Relay` already uses. Multiple `Worker` instances run concurrently against the same `inbox` table with no group-name concept at all; there is no Kafka involvement on this side whatsoever.
  - Each binary still gets its own configurable, uniquely-defaulted Kafka consumer group name regardless (see Section 7) — that default only matters for the `Consumer` axis above.
- No generic multi-topic router — this worker consumes exactly one topic (`email-received`). A future service consuming a different topic gets its own dedicated binary, not a shared multi-topic dispatcher — this is also why naming and env vars are scoped per topic-binary rather than left generic (see Section 1).
- No shared generic "claim + execute + retry + quarantine" engine between `libs/outbox` and `libs/inbox` — deliberately kept as two structurally-similar but independently-implemented packages for now, to avoid touching already-shipped `libs/outbox` logic for a modest amount of duplication. Revisit only if a third similar consumer appears.
- No shared `Record` type between `outbox` and `inbox` either, for the same reason — their shapes aren't actually identical (`outbox.Record` carries a `Key` for partitioning on produce; `inbox.Record` doesn't need one), and unifying just the data shape while keeping the engines independent would be a half-measure.

## 1. Naming convention (preliminary — applied to the existing outbox-relay first)

Every topic-specific binary is named explicitly `<topic-name>-producer` or `<topic-name>-consumer` — never a generic name like `outbox-relay` or `inbox-worker` — because a service may eventually publish or consume several different topics, each getting its own dedicated binary. The convention covers deployable identity *and* configuration surface (per your direction to "touch everything"): folder path, Dockerfile name, Docker image tag, docker-compose service/container name, Taskfile task name, the structured-logger service tag, and env var name prefixes. It does **not** extend to internal Go identifiers (struct/field names like `PollInterval`, or the reusable engine type names `outbox.Relay`/`inbox.Worker`/`inbox.Consumer`) — those stay generic since they're already unambiguous, scoped by their own package/type, and reused across whichever topic-binary wires them up.

**Rename `outbox-relay` → `email-received-producer` (done first, before any inbox code):**

| Before | After |
|---|---|
| `services/email/cmd/outbox-relay/` | `services/email/cmd/email-received-producer/` |
| `services/email/Dockerfile.outbox-relay` | `services/email/Dockerfile.email-received-producer` |
| Taskfile: `build:email:outbox-relay`, image tag `email-outbox-relay` | `build:email:email-received-producer`, image tag `email-received-producer` |
| `dev/docker-compose.yaml`: service/`container_name: email-outbox-relay` | service/`container_name: email-received-producer` |
| `logger.New("outbox-relay")` in `main.go` | `logger.New("email-received-producer")` |
| Env vars: `OUTBOX_POLL_INTERVAL`, `OUTBOX_BATCH_SIZE`, `OUTBOX_MAX_ATTEMPTS`, `OUTBOX_CLAIM_TTL` | `EMAIL_RECEIVED_PRODUCER_POLL_INTERVAL`, `EMAIL_RECEIVED_PRODUCER_BATCH_SIZE`, `EMAIL_RECEIVED_PRODUCER_MAX_ATTEMPTS`, `EMAIL_RECEIVED_PRODUCER_CLAIM_TTL` |

`DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME` and `KAFKA_BROKERS` are unaffected — they aren't topic-specific. The Go struct field names in `config.go` (`PollInterval`, `BatchSize`, etc.) and the `outbox.Relay` type itself are unaffected — only the *string* env var names and file/deployment identifiers change.

**New `email-received-consumer` binary follows the same convention from the start** (see Section 6): folder `services/core/cmd/email-received-consumer/`, `Dockerfile.email-received-consumer`, Taskfile task `build:core:email-received-consumer`, image tag/compose service/container name `email-received-consumer`, `logger.New("email-received-consumer")`, and env vars `EMAIL_RECEIVED_CONSUMER_POLL_INTERVAL`, `EMAIL_RECEIVED_CONSUMER_BATCH_SIZE`, `EMAIL_RECEIVED_CONSUMER_MAX_ATTEMPTS`, `EMAIL_RECEIVED_CONSUMER_CLAIM_TTL`, `EMAIL_RECEIVED_CONSUMER_GROUP_NAME`.

## 2. Architecture overview

```
┌──────────────────────────────────────────────────────────────────┐
│  services/core/cmd/email-received-consumer  (new binary)         │
│                                                                    │
│  ┌─────────────┐        ┌──────────────┐                        │
│  │  Consumer    │        │   Worker     │                        │
│  │ (goroutine)  │        │ (goroutine)  │                        │
│  │              │        │              │                        │
│  │ Kafka poll → │        │ poll inbox → │                        │
│  │ Save(inbox   │        │ claim →      │                        │
│  │  row) →      │        │ Handler() →  │                        │
│  │ commit offset│        │ Mark*        │                        │
│  └──────┬───────┘        └──────┬───────┘                        │
│         │                       │                                 │
│         └───────────┬───────────┘                                 │
│                      ▼                                             │
│              inbox table (Postgres)                                │
│                      ▲                                             │
│         Handler = today's CreateEmailReceivedCallback logic,      │
│         relocated and adapted, now invoked by Worker instead      │
│         of inline in the Kafka poll loop — imports core's own     │
│         RequestService, CategoryService, LLM client directly      │
└──────────────────────────────────────────────────────────────────┘
```

Two independent goroutines in one process, coordinated only through the `inbox` table (same coordination style as `outbox.Relay` ↔ `mails`/`outbox` tables):

- **Consumer**: subscribes to `email-received`; for each fetched record, does *only* `Store.Save(topic, partition, offset, payload)` (idempotent insert, deduped on the Kafka coordinate), then commits the offset. No business logic runs on this path, so Kafka acking is never blocked by a slow LLM call.
- **Worker**: polls the `inbox` table on an interval (mirrors `outbox.Relay` — atomic claim via `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING ...`, `status` state machine `pending → processing → processed | failed`, `claimed_at` + TTL for crash recovery, `MaxAttempts` before quarantine), and for each claimed record invokes an injected `Handler(ctx, record) error`.

This binary fully replaces core's current inline Kafka subscription. `cmd/main.go`'s call to `InitializeEvents`/`SubscribeEmailReceived` is removed entirely; core's HTTP server no longer touches Kafka at all.

Single-instance-for-strict-ordering / scale-for-throughput tradeoff is identical to the outbox relay's: running one `email-received-consumer` instance preserves insertion-order processing; running multiple instances increases throughput at the cost of ordering (no partitioning-by-key scheme exists yet to preserve both, same as outbox).

## 3. `libs/inbox` package

```go
package inbox

// Record is one durably-persisted inbox row, ready for processing.
type Record struct {
    ID      int64
    Topic   string
    Payload []byte
}

// Store is implemented per-service against that service's own inbox table.
// Mirrors outbox.Store's shape, for the opposite direction.
type Store interface {
    // Save durably persists one raw Kafka record, deduped on
    // (topic, partition, offset). inserted=false on a duplicate — a safe
    // no-op, not an error. The caller commits the Kafka offset regardless
    // of inserted's value, since either way this offset is now durably
    // known.
    Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (inserted bool, err error)

    // FetchBatch atomically claims up to limit pending (or expired-claim)
    // records in insertion order. Same FOR UPDATE SKIP LOCKED
    // atomic-UPDATE-with-inner-SELECT requirement as outbox.Store.
    FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]Record, error)

    MarkProcessed(ctx context.Context, id int64) error
    MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error
}

// Handler processes one durably-persisted inbox record. Injected by the
// consuming service — this is where business logic (calling domain
// services, LLM, etc.) lives. Never called until the record is already
// safely persisted.
type Handler func(ctx context.Context, record Record) error

// Consumer reads from Kafka and persists into the inbox, committing the
// offset only after Save succeeds. No Handler is invoked here. Owns its
// own kgo consumer-group client directly — mirrors outbox.Producer's
// independence from libs/events (see section 5), rather than depending
// on a planeo-internal package for Kafka mechanics.
type Consumer struct { /* brokers []string, groupName, topic string, store Store */ }
func NewConsumer(brokers []string, groupName, topic string, store Store) *Consumer
func (c *Consumer) Run(ctx context.Context) error

// Worker polls the inbox and invokes Handler for each claimed record,
// structurally mirroring outbox.Relay (same Option pattern: WithPollInterval,
// WithBatchSize, WithMaxAttempts, WithClaimTTL).
type Worker struct { /* store, handler, pollInterval, batchSize, maxAttempts, claimTTL */ }
func NewWorker(store Store, handler Handler, opts ...Option) *Worker
func (w *Worker) Run(ctx context.Context) error
```

`Save`'s dedup is on the Kafka coordinate `(topic, partition, offset)` — chosen over a payload-derived business key because it requires zero knowledge of what's inside the payload, keeping the library as opaque to payload contents as `outbox.Record.Payload` already is. This protects against exactly the failure this pattern must solve: a message persisted but not yet acked gets redelivered after a crash/restart.

## 4. Postgres schema + `services/core`'s `Store` implementation

New migration in `services/core/internal/infra/postgres/migrations/`:

```sql
-- +goose Up
CREATE TABLE inbox (
    id           BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    topic        TEXT NOT NULL,
    partition    INTEGER NOT NULL,
    "offset"     BIGINT NOT NULL,
    payload      BYTEA NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    claimed_at   TIMESTAMPTZ,
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT,
    received_at  TIMESTAMPTZ DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    failed_at    TIMESTAMPTZ,
    UNIQUE (topic, partition, "offset")
);

CREATE INDEX inbox_pending_idx ON inbox (id) WHERE status IN ('pending', 'processing');
```

Mirrors `outbox`'s schema (same status state machine, same claim/TTL/quarantine columns), with `mail_id`/`key` replaced by `topic, partition, "offset"` as the dedup key (quoted — `offset` is a reserved word in Postgres), and `received_at` replacing `created_at` for naming clarity on the inbound side.

`services/core/internal/infra/postgres/inbox_repository.go` implements `inbox.Store`:
- `Save`: `INSERT INTO inbox (topic, partition, "offset", payload) VALUES (...) ON CONFLICT (topic, partition, "offset") DO NOTHING RETURNING id` — same `pgx.ErrNoRows` → `(false, nil)` pattern as `services/email`'s `mail_repository.go`'s `CreateMail`.
- `FetchBatch`/`MarkProcessed`/`MarkFailed`: same atomic-claim/quarantine SQL shape as `services/email`'s `outbox_repository.go`, reversed direction (claims `pending`/expired-`processing` rows, ordered by `id`).

## 5. `libs/events` is deleted wholesale

**Corrected from an earlier draft of this section.** The first draft had `inbox.Consumer` build on `libs/events.EventService.Subscribe` (extended to expose partition/offset), to avoid re-implementing the Kafka consumer-group polling loop. That broke symmetry with the already-shipped `libs/outbox.Producer`, which faced the identical situation on the producing side — `libs/events.Publish` already existed and did what a producer needs — and deliberately did **not** reuse it: `outbox.NewProducer` creates its own independent `kgo.Client` directly, with zero dependency on `libs/events`. There's no strong reason for `inbox.Consumer` to behave differently; the modest amount of duplicated `kgo` boilerplate (`kgo.NewClient` with `ConsumerGroup`/`ConsumeTopics`/`DisableAutoCommit`, the `PollFetches` loop, `EachRecord`, `CommitRecords`) is the same tradeoff `outbox` already accepted on the other side.

`inbox.Consumer.Run` (Section 3) now owns this logic directly — it's a straight port of `libs/events.Subscribe`'s existing implementation, with the callback replaced by `store.Save`:

```go
func (c *Consumer) Run(ctx context.Context) error {
    client, err := kgo.NewClient(
        kgo.SeedBrokers(c.brokers...),
        kgo.AllowAutoTopicCreation(),
        kgo.ConsumerGroup(c.groupName),
        kgo.ConsumeTopics(c.topic),
        kgo.DisableAutoCommit(),
    )
    if err != nil {
        return err
    }

    log := logger.FromContext(ctx)

    go func() {
        defer client.Close()
        for {
            if ctx.Err() != nil {
                return
            }
            fetches := client.PollFetches(ctx)
            if fetches.IsClientClosed() {
                return
            }
            fetches.EachError(func(_ string, _ int32, err error) {
                log.Error().Err(err).Msg("kafka fetch error")
            })
            fetches.EachRecord(func(record *kgo.Record) {
                if _, err := c.store.Save(ctx, c.topic, record.Partition, record.Offset, record.Value); err != nil {
                    log.Error().Err(err).Msg("failed to persist inbox record, skipping commit")
                    return
                }
                if err := client.CommitRecords(ctx, record); err != nil {
                    log.Error().Err(err).Msg("failed to commit kafka offset")
                }
            })
        }
    }()

    return nil
}
```

**This means `libs/events` (the `service.go`/`email_received.go` files — not the separate `libs/events/contracts` subpackage, which stays and is unaffected) has zero remaining callers anywhere in the repo once this plan lands.** `Publish`/`PublishEmailReceived` were already confirmed dead (`outbox.NewProducer` never used them). `Subscribe`/`SubscribeEmailReceived`/`IsConnected`/`NewEventService`/`Close` lose their only remaining caller once Section 6's rewrite removes `services/core`'s `InitializeEvents` call. Rather than trim two functions and leave a package alive that nothing depends on, `libs/events/service.go` and `libs/events/email_received.go` are deleted wholesale — `libs/events/contracts` is unaffected and continues to be imported directly by both `services/email`'s `domain/mail` (producer side) and `services/core`'s `CreateInboxHandler`/`email-received-consumer` (consumer side).

## 6. Sidecar wiring — `services/core/cmd/email-received-consumer`

The new binary needs only the subset of `cmd/main.go`'s wiring that `CreateEmailReceivedCallback`'s logic actually touches — `categoryService`, `requestService`, the DB client, and the LLM client — not the REST server, Keycloak, or `userService`/`organizationService`:

```go
// services/core/cmd/email-received-consumer/main.go
func main() {
    // logger setup (logger.New("email-received-consumer")), LoadConfig —
    // same pattern as email-received-producer/main.go

    db := postgres.NewClient(ctx, cfg.DatabaseConfig())
    defer db.Close()

    categoryService := category.NewService(db)
    requestService := request.NewService(db)

    handler := coreEvents.CreateInboxHandler(coreEvents.Services{
        RequestService:  requestService,
        CategoryService: categoryService,
    })

    brokers := strings.Split(cfg.KafkaBrokers, ",")
    consumer := inbox.NewConsumer(brokers, cfg.GroupName, contracts.EmailReceivedTopic, db)
    worker := inbox.NewWorker(db, handler,
        inbox.WithPollInterval(cfg.PollInterval),
        inbox.WithBatchSize(cfg.BatchSize),
        inbox.WithMaxAttempts(cfg.MaxAttempts),
        inbox.WithClaimTTL(cfg.ClaimTTL),
    )

    // run both goroutines, wait on signal.NotifyContext (SIGINT/SIGTERM),
    // same shutdown shape as email-received-producer/main.go
}
```

`CreateEmailReceivedCallback` (in `services/core/internal/infra/events/email_received.go`) is renamed/adapted to `CreateInboxHandler`, with its signature changed from `func(payload contracts.EmailCreatedPayload) error` to `func(ctx context.Context, record inbox.Record) error` — it unmarshals `record.Payload` into `contracts.EmailCreatedPayload` itself (mirroring how `mail.OutboxEvent.Payload` is opaque bytes on the producer side), then runs the same body it does today (`CreateRequest` → `GetCategories` → LLM extract/classify → `UpdateRequest`).

`services/core/internal/infra/events/events.go`'s `InitializeEvents` is removed; `cmd/main.go` drops the `coreEvents.InitializeEvents(...)` call and its Kafka-connection-failure `Fatal()` — core's HTTP binary no longer touches Kafka.

Deployment: new `services/core/Dockerfile.email-received-consumer`, `task build:core:email-received-consumer`, and a new `email-received-consumer` service in `dev/docker-compose.yaml` — mirroring `email-received-producer`'s Dockerfile/Taskfile/compose wiring (Section 1).

## 7. Consumer group naming

`inbox.NewConsumer(brokers, groupName, topic, store)` takes `groupName` as an explicit caller-supplied parameter — never hardcoded inside `libs/inbox`. `services/core/cmd/email-received-consumer/config.go` loads it from a new `EMAIL_RECEIVED_CONSUMER_GROUP_NAME` env var, defaulting to `"core-email-received-consumer"` but overridable.

The default is prefixed with the owning service name (`core-`), not just the topic, because Kafka's consumer-group semantics load-balance partitions *within* a group (competing consumers) but broadcast independently *across* distinct groups. If a future service also wants the full `email-received` stream, defaulting both to a bare `"email-received-consumer"` group name would silently collide them into one group and split the stream between them instead of each getting all of it. Prefixing by service name keeps the default collision-free across services in this monorepo, while still being fully overridable per deployment if ever needed.

## 8. Testing

- `libs/inbox`: unit tests using a fake `Store` (mirrors `libs/outbox/relay_test.go`'s `newFakeStore`), covering `Worker`'s claim/handle/mark-processed/mark-failed/quarantine logic without a real DB or Kafka.
- `services/core/internal/test/inbox/inbox_test.go` (new): integration test via testcontainer, mirroring `services/email`'s `mail_test.go`/`outbox_test.go` style — covers `Save`'s dedup on `(topic, partition, offset)`, and `FetchBatch`/`MarkProcessed`/`MarkFailed`'s claim/TTL/quarantine behavior against real Postgres.
- No test for `Consumer` against a real Kafka broker — mirrors `libs/outbox`'s `Producer`: the concrete Kafka-touching type isn't unit-tested, only the pieces around it (`Worker`, `Store`).
- Section 1's rename is a pure identifier/identity change with no logic touched — existing outbox tests aren't expected to need any changes beyond what the rename itself mechanically requires (e.g. any test referencing the old binary path, if one exists).

## 9. Optional: per-key ordering in `Worker` (not designed, not scheduled — ideas only)

**Problem:** `services/email`'s outbox already sets the Kafka record key to the organization ID (`service.go:78`), so same-organization messages always land on the same partition and are ordered relative to each other by Kafka. But `Worker.FetchBatch` claims rows purely by global `id`, with no concept of key — so that per-organization ordering, already correctly preserved all the way into the `inbox` table, is not preserved once more than one `Worker` instance is running. Today's design only gives a real ordering guarantee with exactly one `Worker` instance; running more trades away all ordering, not just cross-organization ordering.

**This is deliberately not designed here — likely premature optimization.** A single `Worker` instance may well provide sufficient throughput indefinitely; this only matters at all if/when horizontal `Worker` scaling is actually needed, which isn't known to be true yet. What follows is raw idea capture for whoever picks this up later, not a spec to implement as written — a first pass (partial unique index on `key WHERE status='processing'`, catching constraint violations as expected races) was worked through during design and found to be broken: that constraint is table-wide, not per-worker, so even a *single* `Worker` batch-claiming multiple rows of the same key in one `UPDATE` would violate its own constraint — and under concurrent access, a second `Worker` can still slip in and claim a same-key row before the first `Worker`'s claim of the rest of that key's batch has committed (read-committed isolation doesn't expose in-flight, uncommitted claims to a concurrent transaction). Getting this genuinely correct likely means a session-held advisory lock per distinct key present in a claimed batch — held for the duration of that `Worker`'s processing of that key's rows, not just at the claim instant — so one `Worker` can freely claim and sequentially process many same-key rows (no self-violation), while any other `Worker` is blocked from that key entirely until the lock releases. That's real concurrency-design work (lock scope, held-lock deadlock/timeout handling, a dedicated concurrency integration test that races real claims against real Postgres) that should happen if and when this is picked up, not decided speculatively now.

## Explicitly deferred / out of scope

- Kafka consumer-group redesign beyond per-binary configurable, collision-safe-by-default group names (see Non-goals).
- Multi-topic support in one worker binary (see Non-goals).
- Shared claim/retry engine, or shared `Record` type, between `libs/outbox` and `libs/inbox` (see Non-goals).
- Any functional change to `services/email`'s outbox pattern or schema — Section 1 is naming-only.
