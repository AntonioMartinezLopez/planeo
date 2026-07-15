# Adapter-Owned Transactional Processing for Outbox/Inbox

## Purpose

`services/email`'s outbox pattern and `services/core`'s inbox pattern (both already shipped) each split their control flow between a reusable lib type (`outbox.Relay`, `inbox.Worker`) that owns the poll loop, the atomic claim, and the mark-processed/mark-failed bookkeeping, and a thin per-service `Store`/`Handler` plugged into it. That structure has two problems:

1. **No explicit ownership.** The lib type, not the service, drives the whole flow — a service that needs more than one producer or consumer (a real near-term need: a service publishing to a second topic, or consuming a second one) has no natural place to hang topic-specific identity, config, or business wiring, since `Relay`/`Worker` are generic and anonymous.
2. **On the consumer side specifically, the claim and the business write are not atomic with each other.** `Worker.pollOnce` claims a batch (one atomic `UPDATE`), then separately calls the handler (which does its own, separately-committing domain-service writes), then separately marks the row processed. If the handler's writes succeed but `MarkProcessed` fails, the row is redelivered later and reprocessed — `services/core`'s Task 5 (`CreateRequest` idempotency) exists specifically to paper over this gap with upsert semantics, rather than the gap being structurally closed.

This spec introduces **adapters**: per-service, per-topic types that own the poll loop and the transactional claim/write/mark flow directly, replacing `outbox.Relay`/`inbox.Worker`. The two existing libs (`libs/outbox`, `libs/inbox`) shrink to their Kafka-only responsibilities (`Producer`, `Consumer`) plus a minimal generic ticker loop; everything claim/mark/transaction-related moves into each service's own adapter and repository.

This spec references, but does not edit, `docs/superpowers/specs/2026-07-10-email-outbox-pattern-design.md` and `docs/superpowers/specs/2026-07-14-inbox-pattern-design.md`. Both of those plans are already complete; this is a follow-up redesign of the control-flow layer they introduced, not a correction of a defect in either.

## Non-goals

- **Does not reduce duplicate Kafka sends on the producer side, and cannot.** Kafka is not enrolled in the Postgres transaction the adapter uses for its own bookkeeping — "produce succeeds, then the process crashes before `MarkProcessed` commits" still redelivers the same row on the next poll, exactly as today. The producer-side value of this redesign is control/ownership and the `claimed_by` ordering fix (Section 3), not a reduction in duplicate-send risk. Consumer-side idempotency (already built, both in `inbox.Save`'s dedup and Task 5's `CreateRequest` upsert) remains the only real protection against the resulting duplicate, unchanged.
- **Does not support multiple concurrent instances of the same producer/consumer.** Both sides remain single-instance-per-topic for ordering reasons, same as today. The `claimed_by` mechanism (Section 3) is safe if a second instance is ever added later, but does not restore strict ordering across concurrent instances — that remains the already-deferred "per-key ordering" idea from the original inbox-pattern spec's Section 9.
- **No change to `services/email`'s or `services/core`'s domain logic** beyond what's needed for `CreateRequest`/`UpdateRequest` to participate in a caller-supplied transaction (Section 4). `CreateRequest`'s idempotent-upsert behavior (Task 5) is unchanged and remains in place as defense-in-depth against genuine duplicate Kafka produces — this redesign closes the *`MarkProcessed`-fails-after-a-successful-handler* gap specifically, not the *Kafka redelivers an already-fully-processed message* case, which idempotency alone still handles.
- **No multi-topic router, no shared claim/retry engine between the outbox and inbox sides.** Same boundaries as both original specs — this redesign changes *where* the claim/retry logic lives (adapter instead of lib), not the "two independently-implemented packages" boundary between outbox and inbox themselves.

## 1. `libs/outbox` and `libs/inbox`: what shrinks, what stays

Both libs lose all claim/mark/attempts/transaction logic. What remains:

**`libs/outbox`:**
- `Producer`/`kafkaProducer`/`NewProducer` (raw Kafka send) — unchanged.
- `Relay` — removed. Replaced by a minimal `Runner`:
  ```go
  type Runner struct { pollInterval time.Duration }
  func NewRunner(opts ...Option) *Runner
  func (r *Runner) Run(ctx context.Context, poll func(context.Context) error) error
  // same ticker + ctx.Done() loop Relay.Run has today, with no Store/Producer coupling
  ```
- `Store` interface — removed from the lib entirely. Its contract moves into each service's adapter package (Section 2), generalized to serve multiple producers.

**`libs/inbox`:**
- `Consumer` (Kafka-receive → `Save` → offset-commit) — unchanged. This side isn't affected by the redesign; it already durably persists before acking, which remains correct as-is.
- `Worker` — removed. Replaced by the same minimal `Runner` shape as `libs/outbox` (a separate implementation, not shared — preserves the existing "independently-implemented" boundary between the two libs).
- `Store` interface and `Handler` type — removed from the lib. Both become `services/core`-adapter-internal concepts (Section 4).

## 2. `services/email`'s producer adapters

New package `services/email/internal/infra/outbox/`. The repository interface is declared here, at the point of use (not in the lib), and is shared by every producer adapter in the service — adding a second producer for a different topic later means constructing a second adapter against the same repository, not writing new persistence code.

```go
package outbox // services/email/internal/infra/outbox

// Repository is implemented once per service (postgres.OutboxRepository), shared by
// every producer adapter. topic/instanceID are passed per call so one repository
// instance can serve any number of topic-scoped adapters.
type Repository interface {
    // FetchBatch atomically claims up to limit rows for topic that are either
    // pending, or processing-and-claimed-by-this-instance (fast self-reclaim,
    // see Section 3), or processing-with-an-expired-claim (crash-recovery
    // fallback). Same FOR UPDATE SKIP LOCKED atomic-UPDATE-with-inner-SELECT
    // requirement as today's outbox.Store.FetchBatch.
    FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]Record, error)
    MarkProcessed(ctx context.Context, id int64) error
    MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error
}
```

Concrete implementation, `services/email/internal/infra/postgres/outbox_repository.go`:
```go
type OutboxRepository struct { pool *pgxpool.Pool }
func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository { ... }
```
One shared instance, constructed once in `main.go` (or once per binary), reused by every producer adapter.

**Adapter — explicitly named per topic** (a service with multiple producers gets one adapter type per producer, not one generic parameterized type):
```go
type EmailReceivedProducerAdapter struct {
    repo        Repository
    producer    outbox.Producer
    topic       string
    instanceID  string // generated once at process startup (e.g. a UUID)
    batchSize   int
    claimTTL    time.Duration
    maxAttempts int
}

func (a *EmailReceivedProducerAdapter) pollOnce(ctx context.Context) error {
    records, err := a.repo.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
    if err != nil {
        return err
    }
    for _, rec := range records {
        if err := a.producer.ProduceSync(ctx, rec.Topic, rec.Key, rec.Payload); err != nil {
            _ = a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
            continue
        }
        _ = a.repo.MarkProcessed(ctx, rec.ID)
    }
    return nil
}
```
This is functionally close to today's `Relay.pollOnce` — no transaction wraps `ProduceSync` (there is no atomicity benefit available; see Non-goals), and the known "produce succeeds, mark fails → resend" gap is unchanged. What changes is *where* this logic lives (a named, per-topic adapter, not a generic lib type) and the `claimed_by`-based fix below.

Wired in `cmd/email-received-producer/main.go`:
```go
repo := postgres.NewOutboxRepository(db.Pool())
adapter := outbox.NewEmailReceivedProducerAdapter(repo, producer, contracts.EmailReceivedTopic, generateInstanceID(), ...)
outbox.NewRunner(...).Run(ctx, adapter.pollOnce)
```

## 3. The `claimed_by` fix: order preservation under a single instance

**The problem, independent of any of the above:** even with exactly one producer instance, if `ProduceSync` for row 5 succeeds but the subsequent `MarkProcessed` fails, row 5 is left at `status='processing'`. It is not reclaimable again until `claimTTL` (e.g. 30s) elapses — but every poll tick in between claims and produces *newer* pending rows (6, 7, 8, ...) ahead of it. Kafka ends up seeing 6, 7, 8, ... before eventually seeing 5 again (as a duplicate, once the TTL window closes) — order is broken by a single instance, with no concurrency involved at all.

**The fix:** since the deployment is single-instance-per-topic, that same instance can safely reclaim its *own* stuck row immediately, without waiting for `claimTTL` — sequential polling guarantees no other in-flight attempt is still touching it (the previous poll tick already returned before this one started). `claimTTL` remains necessary only as a fallback for the case where the instance itself crashed and a *new* process (a new instance id) needs to eventually recover orphaned rows left by the old one.

**Schema addition** (both `outbox` and `inbox` tables): `claimed_by TEXT` (nullable), set alongside `claimed_at`/`status='processing'` in the claiming `UPDATE`.

**`FetchBatch`'s claim query**, conceptually:
```sql
UPDATE outbox SET status = 'processing', claimed_at = NOW(), claimed_by = @instanceId
WHERE id IN (
  SELECT id FROM outbox
  WHERE status = 'pending'
     OR (status = 'processing' AND claimed_by = @instanceId)          -- fast self-reclaim, no TTL wait
     OR (status = 'processing' AND claimed_at < NOW() - @claimTTL)    -- crashed-instance fallback
  ORDER BY id
  FOR UPDATE SKIP LOCKED
  LIMIT @limit
)
RETURNING *
```
(Same shape for `inbox`'s equivalent query — see Section 4.)

**Explicitly documented limitation:** this preserves order and enables immediate self-reclaim correctly under a single instance, and remains *safe* (no corruption, no double-claim — `SKIP LOCKED` plus the per-instance `claimed_by = @instanceId` clause means one instance never fast-reclaims another instance's in-flight claim) if a second concurrent instance is ever added later. It does **not** restore strict topic-wide ordering once more than one instance is producing concurrently — `SKIP LOCKED` still lets different instances claim different rows from the same batch independently, racing to produce them in whatever order each happens to finish. That remains the deferred "per-key ordering" idea from the original inbox-pattern spec's Section 9.

`instanceID` is generated once per process at startup (e.g. a UUID) — not user-configured, not persisted anywhere beyond the rows it claims.

## 4. `services/core`'s consumer adapter

Mirrors the producer's claim mechanism exactly (same schema fields, same `FetchBatch` query shape, same `claimed_by` fast-reclaim), but wraps the handler's own DB writes and the final mark in one transaction per record — closing the `MarkProcessed`-fails-after-a-successful-handler gap structurally, without holding that transaction open across the handler's LLM calls.

**Schema:** the `inbox` table (from the original inbox-pattern spec) keeps `status`(`pending`/`processing`/`processed`/`failed`)/`claimed_at`/`attempts`/`last_error`/`failed_at`, and gains `claimed_by TEXT`, mirroring the outbox table exactly.

**Repository — declared at point of use, includes `WithTransaction` for the write-only phase** (named to match the existing convention already established by `services/email`'s `mail_repository.go`'s `Client.WithTransaction`, not the lib-level `libs/db.WithTx` helper it wraps):
```go
package inbox // services/core/internal/infra/inbox

type Repository interface {
    FetchBatch(ctx context.Context, instanceID string, limit int, claimTTL time.Duration) ([]Record, error)
    WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
    MarkProcessed(ctx context.Context, id int64) error
    MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error
}
```
`*postgres.Client`'s `WithTransaction` wraps `libs/db.WithTx(ctx, c.db, fn)` — the adapter never touches `*pgxpool.Pool` or `libs/db` directly, only this interface. `FetchBatch` stays on the plain pool (it's never called inside the adapter's transaction). **`MarkProcessed`/`MarkFailed` must resolve their `Querier` via `db.FromContext(ctx, c.db)`, not call `c.db` directly** — otherwise, called with `txCtx` from inside `WithTransaction`, they'd silently run on a separate, auto-committed connection instead of participating in the transaction, defeating the entire point of this section.

**Adapter — batch claim (rows already carry their full `Payload`), then per record: gather outside any transaction, write inside a short one:**
```go
type EmailReceivedConsumerAdapter struct {
    repo            Repository
    requestService  request.Service
    categoryService category.Service
    instanceID      string
    batchSize       int
    claimTTL        time.Duration
    maxAttempts     int
}

func (a *EmailReceivedConsumerAdapter) pollOnce(ctx context.Context) error {
    records, err := a.repo.FetchBatch(ctx, a.instanceID, a.batchSize, a.claimTTL)
    if err != nil {
        return err
    }
    for _, rec := range records {
        a.processRecord(ctx, rec) // logs its own errors; one bad record doesn't stop the batch
    }
    return nil
}

func (a *EmailReceivedConsumerAdapter) processRecord(ctx context.Context, rec Record) error {
    var payload contracts.EmailCreatedPayload
    if err := json.Unmarshal(rec.Payload, &payload); err != nil {
        return a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
    }

    categories, err := a.categoryService.GetCategories(ctx, payload.OrganizationId)
    if err != nil {
        return a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
    }
    extractedFields, _ := llm.ExtractRequestFields(ctx, raw) // failure logged, not fatal - unchanged existing behavior
    categoryId, err := llm.ClassifyRequest(ctx, requestData, categories)
    if err != nil {
        return a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
    }

    // Write-only transaction: CreateRequest/UpdateRequest's writes and MarkProcessed are atomic
    // together. MarkFailed is deliberately NOT called inside this transaction: once any statement
    // in a Postgres transaction errors, that transaction is aborted and every subsequent statement
    // on it fails too (SQLSTATE 25P02) - so calling MarkFailed(txCtx, ...) after CreateRequest's own
    // error would itself fail, WithTransaction would roll back, and the attempts increment would be
    // lost entirely. A row that deterministically fails would retry forever and never quarantine.
    txErr := a.repo.WithTransaction(ctx, func(txCtx context.Context) error {
        requestId, err := a.requestService.CreateRequest(txCtx, request.NewRequest{ /* ... */ })
        if err != nil {
            return err
        }
        if err := a.requestService.UpdateRequest(txCtx, request.UpdateRequest{ /* ...extractedFields, categoryId... */ }); err != nil {
            return err
        }
        return a.repo.MarkProcessed(txCtx, rec.ID)
    })
    if txErr != nil {
        return a.repo.MarkFailed(ctx, rec.ID, txErr, a.maxAttempts) // separate call, on the plain ctx, after rollback
    }
    return nil
}
```

**Why gather-then-write, not one transaction around everything:** the handler makes two sequential LLM calls (`ExtractRequestFields`, `ClassifyRequest`), each potentially seconds long. Wrapping those in the same transaction as the row lock would hold a Postgres row lock and a pooled connection for the full round-trip of each call — tolerable at this project's volume, but an unnecessary, avoidable cost. Gathering first means the transaction only spans the parts that actually need to be atomic together: the two domain writes and the inbox row's final status.

**On partial-write handling:** if `CreateRequest` succeeds but `UpdateRequest` then fails, the whole transaction rolls back — the partially-created `Request` row does not survive. This is not a loss compared to preserving it: the LLM calls already happened *before* the transaction, so a retry redoes that gathering work regardless of whether a partial row existed; Task 5's idempotent upsert means the retry's `CreateRequest` call ends up at the same row either way. Rolling back everything and recording the failure in a separate call after the fact is both the only mechanism Postgres actually permits here (see above) and loses nothing the alternative would have preserved.

**Requires:** `services/core`'s `request`/`category` domain-service repositories (specifically `CreateRequest`/`UpdateRequest`'s write path) to resolve their `Querier` via `db.FromContext(ctx, pool)` instead of calling `c.db` directly, mirroring the pattern `services/email`'s `mail`/`outbox` repositories already use (from the earlier outbox-architecture-cleanup plan). Today, `services/core`'s repositories ignore any transaction on `ctx` entirely — this is a real, contained migration, scoped to the write path these two methods use, not a wholesale rewrite of `services/core`'s persistence layer.

## 5. Config

Both adapters need, per topic: `batchSize`, `claimTTL`, `maxAttempts`, and a process-generated `instanceID` (not user-configured). Existing env vars (`EMAIL_RECEIVED_PRODUCER_*`, `EMAIL_RECEIVED_CONSUMER_*`) are unchanged — `claimTTL` remains a live, load-bearing config value on both sides (its earlier proposed removal, from an intermediate draft of this design, is reverted: the batch-claim model needs it as the crashed-instance recovery fallback described in Section 3).

## 6. Testing

- **`libs/outbox`/`libs/inbox`**: shrink to `Producer`/`Consumer` + `Runner`. Existing `Relay`/`Worker`/`Store`/`Handler` unit tests are removed along with the types they tested; `Runner`'s own test coverage is just its ticker/ctx.Done() loop.
- **`EmailReceivedProducerAdapter`**: new unit test against a fake `Repository` (same fake-store pattern the original `libs/inbox` `worker_test.go` already used) — covering successful produce/process and produce/handler failure → `MarkFailed` called with the right error.
- **`EmailReceivedConsumerAdapter`**: no unit test — `processRecord` calls `llm.ExtractRequestFields`/`llm.ClassifyRequest` directly as free functions, not through an injectable port, so a unit test exercising it would make live network calls to the real Mistral API and require `MISTRAL_API_KEY` in the process environment, which isn't appropriate for a hermetic unit test. This was tried during implementation and retracted once that dependency surfaced. Coverage of this adapter's transaction/rollback behavior (`WithTransaction`'s callback failing → `MarkFailed` called on the plain `ctx` afterward, not inside the failed transaction) lives at the integration level instead (the rollback-atomicity test against real Postgres), which necessarily also exercises the real LLM calls. Revisit adding a unit test only if the LLM calls become injectable.
- **Repositories** (`*postgres.Client` on both services): integration tests via the existing testcontainer harnesses, covering the new `claimed_by`-based behavior specifically — a `processing` row owned by *this* instance is reclaimed immediately (no `claimTTL` wait), while a `processing` row owned by a *different* instance id is only reclaimed after `claimTTL` elapses.
- **`services/core`'s consumer-adapter rollback atomicity** (new, mirroring the outbox-architecture-cleanup plan's "`CreateOutboxEvent` failure rolls back `CreateMail`" integration test): force `UpdateRequest` to fail after a successful `CreateRequest` inside the adapter's transaction, then assert against real Postgres that no `Request` row persists and the inbox row is back to `pending` with `attempts` incremented — proving the rollback is real, not just believed correct by inspection.
- **`services/core`'s `request` repository**: a test proving `CreateRequest`/`UpdateRequest` participate in a caller-supplied transaction via `db.FromContext`, mirroring `libs/db`'s own `TestWithTx`/`TestFromContext` and the `services/email` mail/outbox atomicity integration test from the outbox-architecture-cleanup plan.

## Explicitly deferred / out of scope

- Reducing duplicate Kafka sends on the producer side — structurally not achievable without a much larger mechanism (e.g. a Kafka-transactional producer coordinated with the DB commit); out of scope given current volume and existing consumer-side idempotency (see Non-goals).
- Multiple concurrent instances of the same producer/consumer, and the strict-ordering-under-concurrency problem that comes with it — the `claimed_by` mechanism is safe under future multi-instance use but does not solve ordering; that remains the original inbox-pattern spec's deferred Section 9 idea.
- Any change to `services/email`'s or `services/core`'s domain logic beyond the `Querier`/`FromContext` migration needed for `CreateRequest`/`UpdateRequest` to join the adapter's transaction.
