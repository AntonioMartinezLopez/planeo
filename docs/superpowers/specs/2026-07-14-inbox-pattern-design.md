# Inbox Pattern for services/core's Kafka Consumer

## Purpose

`services/email`'s outbox pattern solved the producer-side dual-write problem (IMAP mark-seen vs. Kafka publish) and now durably guarantees at-least-once delivery of `email-received` events. `services/core`'s consumer side has the mirror-image problem: today, `libs/events.Subscribe` runs the entire business-logic callback (`CreateEmailReceivedCallback` — create a `Request`, call the LLM for field extraction and classification, update the `Request`) synchronously inline in the Kafka poll loop, committing the offset only if that callback succeeds. This has two problems:

1. **No idempotency.** If the callback partially succeeds (e.g. the `Request` is created, but the process crashes before the LLM classification step completes and the offset is committed), Kafka redelivers the same message on restart, and the callback runs again from scratch — creating a duplicate `Request` or repeating side effects, with no protection against it.
2. **Slow processing blocks acking.** LLM extraction and classification are slow, network-dependent calls. Because they run inline before the offset commits, a slow or flaky LLM call directly risks Kafka consumer-group timeouts (`max.poll.interval.ms`) and delays how quickly the consumer group's lag drains.

This spec introduces the **Inbox pattern** — a `libs/outbox`-style reusable engine (`libs/inbox`) plus a new dedicated binary, `services/core/cmd/inbox-worker`, that:
- Durably persists each raw Kafka record into an `inbox` table before committing the offset (deduped on the Kafka coordinate), decoupling "safely acknowledged" from "fully processed."
- Processes persisted inbox records asynchronously, in insertion order, via an injected handler — mirroring the outbox relay's claim/retry/quarantine mechanics in reverse.

This fully replaces `services/core`'s current inline Kafka subscription. It does not change anything on the producer (`services/email`) side.

## Non-goals

- No change to `services/email`'s outbox pattern, its schema, or its relay.
- No redesign of Kafka consumer-group semantics beyond what's needed here — the existing single-global-consumer-group-per-topic model (flagged as a known simplification during the original NATS→Kafka migration) is unchanged; each binary just gets its own configurable group name (see "Consumer group naming" below).
- No generic multi-topic router — this worker consumes exactly one topic (`email-received`). A future service consuming a different topic gets its own dedicated binary, not a shared multi-topic dispatcher.
- No shared generic "claim + execute + retry + quarantine" engine between `libs/outbox` and `libs/inbox` — deliberately kept as two structurally-similar but independently-implemented packages for now, to avoid touching already-shipped `libs/outbox` code for a modest amount of duplication. Revisit only if a third similar consumer appears.

## 1. Architecture overview

```
┌─────────────────────────────────────────────────────────────┐
│  services/core/cmd/inbox-worker  (new binary)                │
│                                                                │
│  ┌─────────────┐        ┌──────────────┐                    │
│  │  Consumer    │        │   Worker     │                    │
│  │ (goroutine)  │        │ (goroutine)  │                    │
│  │              │        │              │                    │
│  │ Kafka poll → │        │ poll inbox → │                    │
│  │ Save(inbox   │        │ claim →      │                    │
│  │  row) →      │        │ Handler() →  │                    │
│  │ commit offset│        │ Mark*        │                    │
│  └──────┬───────┘        └──────┬───────┘                    │
│         │                       │                             │
│         └───────────┬───────────┘                             │
│                      ▼                                         │
│              inbox table (Postgres)                            │
│                      ▲                                         │
│         Handler = today's CreateEmailReceivedCallback logic,  │
│         relocated and adapted, now invoked by Worker instead  │
│         of inline in the Kafka poll loop — imports core's own │
│         RequestService, CategoryService, LLM client directly  │
└─────────────────────────────────────────────────────────────┘
```

Two independent goroutines in one process, coordinated only through the `inbox` table (same coordination style as `outbox.Relay` ↔ `mails`/`outbox` tables):

- **Consumer**: subscribes to `email-received`; for each fetched record, does *only* `Store.Save(topic, partition, offset, payload)` (idempotent insert, deduped on the Kafka coordinate), then commits the offset. No business logic runs on this path, so Kafka acking is never blocked by a slow LLM call.
- **Worker**: polls the `inbox` table on an interval (mirrors `outbox.Relay` — atomic claim via `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING ...`, `status` state machine `pending → processing → processed | failed`, `claimed_at` + TTL for crash recovery, `MaxAttempts` before quarantine), and for each claimed record invokes an injected `Handler(ctx, record) error`.

This binary fully replaces core's current inline Kafka subscription. `cmd/main.go`'s call to `InitializeEvents`/`SubscribeEmailReceived` is removed entirely; core's HTTP server no longer touches Kafka at all.

Single-instance-for-strict-ordering / scale-for-throughput tradeoff is identical to the outbox relay's: running one `inbox-worker` instance preserves insertion-order processing; running multiple instances increases throughput at the cost of ordering (no partitioning-by-key scheme exists yet to preserve both, same as outbox).

## 2. `libs/inbox` package

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
// offset only after Save succeeds. No Handler is invoked here. Built on
// top of libs/events.EventService.Subscribe (see section 4) rather than
// wrapping its own kgo consumer-group client.
type Consumer struct { /* wraps an *events.EventService + groupName + topic + Store */ }
func NewConsumer(eventService events.EventServiceInterface, groupName, topic string, store Store) *Consumer
func (c *Consumer) Run(ctx context.Context) error

// Worker polls the inbox and invokes Handler for each claimed record,
// structurally mirroring outbox.Relay (same Option pattern: WithPollInterval,
// WithBatchSize, WithMaxAttempts, WithClaimTTL).
type Worker struct { /* store, handler, pollInterval, batchSize, maxAttempts, claimTTL */ }
func NewWorker(store Store, handler Handler, opts ...Option) *Worker
func (w *Worker) Run(ctx context.Context) error
```

`Save`'s dedup is on the Kafka coordinate `(topic, partition, offset)` — chosen over a payload-derived business key because it requires zero knowledge of what's inside the payload, keeping the library as opaque to payload contents as `outbox.Record.Payload` already is. This protects against exactly the failure this pattern must solve: a message persisted but not yet acked gets redelivered after a crash/restart.

## 3. Postgres schema + `services/core`'s `Store` implementation

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

## 4. `libs/events.Subscribe` extension

Small, additive change to `libs/events/service.go` so `Consumer` can build on it instead of duplicating Kafka consumer-group plumbing:

```go
func (es *EventService) Subscribe(ctx context.Context, groupName string, topic string, handler func(partition int32, offset int64, data []byte) error) error {
    // ... unchanged setup ...
    fetches.EachRecord(func(record *kgo.Record) {
        if err := handler(record.Partition, record.Offset, record.Value); err != nil {
            // ... unchanged error handling ...
        }
        // ... unchanged commit ...
    })
}
```

`inbox.Consumer` becomes a thin adapter: construct with an `EventServiceInterface`, call `Subscribe(groupName, topic, func(partition int32, offset int64, data []byte) error { return store.Save(ctx, topic, partition, offset, data) })`.

**Cleanup as part of this same change:** `libs/events.SubscribeEmailReceived` and its hardcoded `subscriptionName = "email-receiver"` package var (in `libs/events/email_received.go`) are removed — `services/core` (their only caller) migrates entirely to the inbox worker and stops calling them. `PublishEmailReceived` and the low-level `Subscribe` are unaffected and remain in use (`Consumer` builds on `Subscribe`; `services/email`'s outbox relay is unaffected, it only ever called `Publish`, never `Subscribe`).

## 5. Sidecar wiring — `services/core/cmd/inbox-worker`

The new binary needs only the subset of `cmd/main.go`'s wiring that `CreateEmailReceivedCallback`'s logic actually touches — `categoryService`, `requestService`, the DB client, and the LLM client — not the REST server, Keycloak, or `userService`/`organizationService`:

```go
// services/core/cmd/inbox-worker/main.go
func main() {
    // logger setup, LoadConfig — same pattern as outbox-relay/main.go

    db := postgres.NewClient(ctx, cfg.DatabaseConfig())
    defer db.Close()

    categoryService := category.NewService(db)
    requestService := request.NewService(db)

    eventService, err := events.NewEventService(cfg.KafkaBrokers)
    // ...

    handler := coreEvents.CreateInboxHandler(ctx, coreEvents.Services{
        RequestService:  requestService,
        CategoryService: categoryService,
    })

    consumer := inbox.NewConsumer(eventService, cfg.KafkaConsumerGroup, "email-received", db)
    worker := inbox.NewWorker(db, handler,
        inbox.WithPollInterval(cfg.PollInterval),
        inbox.WithBatchSize(cfg.BatchSize),
        inbox.WithMaxAttempts(cfg.MaxAttempts),
        inbox.WithClaimTTL(cfg.ClaimTTL),
    )

    // run both goroutines, wait on signal.NotifyContext (SIGINT/SIGTERM),
    // same shutdown shape as outbox-relay/main.go
}
```

`CreateEmailReceivedCallback` (in `services/core/internal/infra/events/email_received.go`) is renamed/adapted to `CreateInboxHandler`, with its signature changed from `func(payload contracts.EmailCreatedPayload) error` to `func(ctx context.Context, record inbox.Record) error` — it unmarshals `record.Payload` into `contracts.EmailCreatedPayload` itself (mirroring how `mail.OutboxEvent.Payload` is opaque bytes on the producer side), then runs the same body it does today (`CreateRequest` → `GetCategories` → LLM extract/classify → `UpdateRequest`).

`services/core/internal/infra/events/events.go`'s `InitializeEvents` is removed; `cmd/main.go` drops the `coreEvents.InitializeEvents(...)` call and its Kafka-connection-failure `Fatal()` — core's HTTP binary no longer touches Kafka.

Deployment: new `services/core/Dockerfile.inbox-worker`, `task build:core:inbox-worker`, and a new `core-inbox-worker` service in `dev/docker-compose.yml` — mirroring `outbox-relay`'s existing Dockerfile/Taskfile/compose wiring.

## 6. Consumer group naming

`inbox.NewConsumer(eventService, groupName, topic, store)` takes `groupName` as an explicit caller-supplied parameter — never hardcoded inside `libs/inbox`. `services/core/cmd/inbox-worker/config.go` loads it from a new `KAFKA_CONSUMER_GROUP` env var (mirroring the `OUTBOX_*` env var pattern already used by `outbox-relay/config.go`), defaulting to `"core-inbox-worker"` but overridable.

This matters for future services: Kafka's consumer-group semantics load-balance partitions *within* a group (competing consumers) but broadcast independently *across* distinct groups. A future service that also wants the full `email-received` stream (or any other topic) needs its own distinct group name in its own binary's config — never the same group name as an existing consumer of that topic, or it would only see a load-balanced subset. Because `groupName` is already a plain config value per binary, this requires no library change — just each new service's own default group name.

## 7. Testing

- `libs/inbox`: unit tests using a fake `Store` (mirrors `libs/outbox/relay_test.go`'s `newFakeStore`), covering `Worker`'s claim/handle/mark-processed/mark-failed/quarantine logic without a real DB or Kafka.
- `services/core/internal/test/inbox/inbox_test.go` (new): integration test via testcontainer, mirroring `services/email`'s `mail_test.go`/`outbox_test.go` style — covers `Save`'s dedup on `(topic, partition, offset)`, and `FetchBatch`/`MarkProcessed`/`MarkFailed`'s claim/TTL/quarantine behavior against real Postgres.
- No test for `Consumer` against a real Kafka broker — mirrors `libs/outbox`'s `Producer`, the concrete Kafka-touching type isn't unit-tested, only the pieces around it (and `Consumer` itself is now a thin adapter over the already-used `Subscribe`).

## Explicitly deferred / out of scope

- Kafka consumer-group redesign beyond per-binary configurable group names (see Non-goals).
- Multi-topic support in one worker binary (see Non-goals).
- Shared claim/retry engine between `libs/outbox` and `libs/inbox` (see Non-goals).
- Any change to `services/email`'s outbox pattern or schema.
