# Transactional Outbox Pattern for services/email

## Purpose

Today, `services/email` fetches unseen mail via IMAP, marks each fetched message `\Seen` on the mail server, and only afterward publishes a Kafka event for each one (fire-and-forget; publish errors are logged but not propagated). This is a genuine data-loss bug, not a theoretical one: `imap_service.go`'s `FetchAllUnseenMails` calls IMAP `Store` to add the `\Seen` flag *before* `email_service.go` ever attempts to publish to Kafka. If the Kafka publish fails, or the process crashes between these two steps, the message is already marked read on the mail server and will never be fetched again — the event is lost permanently with no recovery path.

This spec introduces a transactional outbox: the email service durably records every fetched mail and its corresponding outbox event in one local Postgres transaction *before* telling IMAP to mark anything seen. A separate, independent sidecar process drains the outbox to Kafka with retries. This guarantees at-least-once delivery of the Kafka event for every mail the service commits to having fetched, and makes the pattern reusable for future services via a small shared library.

## Non-goals

- No Kafka-side dead-letter topic. A `failed_at` marker on the outbox row (a "poison-row quarantine," not a classic consumer dead-letter queue — see Terminology note below) is sufficient for this pass.
- No message headers / trace-context propagation. Nothing needs them yet; adding a column later is a cheap migration.
- No outbox table cleanup/retention job. Documented as deferred below, not silently forgotten.
- No schema-registry integration itself. The `bytea` payload column is what makes that addable later without a further migration — the registry work itself is out of scope here.
- No changes to `services/core`'s Kafka *subscribe* side (`libs/events.Subscribe`). Its existing log-and-skip-commit failure behavior, and its lack of a consumer-side dead-letter topic, are unrelated, already-deferred concerns from the NATS-to-Kafka migration spec, not part of this work.

**Terminology note:** a classic Kafka "dead-letter queue" is consumer-side — a consumer that fails to *process* a received message routes it to a separate dead-letter *topic* after retries, so the main topic keeps flowing. What this spec calls a "poison-row quarantine" is different: it's the *relay's own send to Kafka* failing repeatedly (broker unreachable, topic auth error, etc.), tracked entirely within the outbox table itself — no second Kafka topic involved.

## Architecture Overview

Three components, delivered together as one cohesive change (they don't function independently):

1. **`libs/outbox`** — a reusable, service-agnostic relay engine. Polls a small `Store` interface (implemented per-service against that service's own outbox table), produces each row's pre-serialized payload bytes to Kafka via franz-go, and marks rows processed or quarantined. This is the part designed for reuse by future services beyond email.
2. **`services/email` write-path changes** — new `mails` and `outbox` tables; a new `domain/mail` package (mirroring the existing `domain/setting` package) whose repository performs the atomic dual-insert; `imap_service.go` split so fetching and mark-`\Seen` become separate steps.
3. **`services/email/cmd/outbox-relay`** — a new, separate binary wiring `libs/outbox`'s generic engine to email's concrete Postgres outbox table and Kafka broker. Its own minimal config, its own Dockerfile, its own docker-compose service.

## Data Model

Both tables live in the email service's existing `mail` Postgres database, added via a new goose migration in the existing `services/email/internal/infra/postgres/migrations/` directory (run through the existing `task migrate:email`).

### `mails`

```sql
CREATE TABLE mails (
    id              INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    message_id      TEXT NOT NULL,
    setting_id      INTEGER NOT NULL REFERENCES settings(id),
    organization_id INTEGER NOT NULL,
    subject         TEXT NOT NULL,
    sender          TEXT NOT NULL,
    body            TEXT NOT NULL,
    date            TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (setting_id, message_id)
);
```

`mails` is the durable source-of-truth for every fetched email (full content, not just a dedup marker), so it also doubles as an audit trail / replay source independent of Kafka or the outbox. The `UNIQUE (setting_id, message_id)` constraint is the crash-safety mechanism: a re-fetch of an already-recorded message (e.g. after a crash between the DB commit and the IMAP mark-seen call) becomes a harmless `ON CONFLICT DO NOTHING`.

### `outbox`

```sql
CREATE TABLE outbox (
    id           BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    mail_id      INTEGER NOT NULL REFERENCES mails(id),
    topic        TEXT NOT NULL,
    key          BYTEA,
    payload      BYTEA NOT NULL,
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT,
    processed_at TIMESTAMPTZ,
    failed_at    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX outbox_pending_idx ON outbox (id) WHERE processed_at IS NULL AND failed_at IS NULL;
```

- `mail_id` is a concrete foreign key to `mails(id)` rather than a generic Debezium-style `aggregate_type`/`aggregate_id` text pair. Each service's outbox table is local to that service's own database anyway, and the reusable `libs/outbox` relay never queries `mails` directly — it only interacts via the `Store` interface. A real FK costs nothing here and gives referential integrity for free.
- `payload` is `bytea`, holding the fully-serialized event bytes (JSON today, same `EmailCreatedPayload` shape as the current NATS/Kafka payload) written at insert time by the email service. The relay never deserializes or understands this payload — it only ships the bytes as the Kafka record value. This is deliberately modeled on Debezium's own outbox-router convention, which recommends `bytea` specifically once a service uses binary/schema-registry serialization (Avro/Protobuf's Confluent wire format is not valid JSON and cannot live in a `jsonb` column). Starting with `bytea` costs nothing now and avoids a future migration when schema-registry work begins.
- `key` is `bytea`, holding the Kafka record key (`[]byte(strconv.Itoa(organizationId))`). This closes a gap explicitly deferred in the earlier NATS-to-Kafka migration spec (no message key existed there), giving per-organization partition ordering once the topic has more than one partition.
- `attempts` / `last_error` / `failed_at` implement the poison-row quarantine: the relay increments `attempts` and records `last_error` on every failed send; once `attempts >= OUTBOX_MAX_ATTEMPTS`, it sets `failed_at` and the row is excluded from further polling (via the partial index and query predicate) but retained for operator inspection.
- The partial index matches the relay's poll query exactly (`WHERE processed_at IS NULL AND failed_at IS NULL ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`), keeping polling cheap as the table grows. `id` is a `BIGINT IDENTITY` so rows are polled in roughly the order they were committed.

**Ordering guarantee, precisely stated:** this is *not* a strict global FIFO guarantee, for two independent reasons, and the design accepts both. First, `GENERATED ALWAYS AS IDENTITY` sequence values can become visible out of commit order (a higher `id` can commit and be polled before a lower, still-uncommitted `id` becomes visible) — at this service's volume this is a non-issue in practice, but it means "ordered" means "best-effort, per-key," not "guaranteed global sequence." Second, and more importantly, the poison-row quarantine *deliberately breaks order*: once a row hits `failed_at`, the relay skips past it and continues with later rows rather than blocking the whole queue on one broken message — so a message can be delivered to Kafka after a quarantined row that was fetched earlier. The actual guarantee this design provides is **best-effort per-organization ordering** (via the `key` column, once the topic has multiple partitions), not strict FIFO across the whole outbox.

## Write Path (services/email)

**`imap_service.go`** splits the current `FetchAllUnseenMails` into two steps, each opening its own IMAP session (login/logout), and switches from sequence numbers to UIDs:
- `FetchUnseenMails(ctx, settings) ([]Email, error)` — uses `UIDSearch` + `Fetch` (an IMAP UID-based fetch) instead of today's sequence-number-based `Search`/`Fetch`, no `\Seen` marking. Each returned `Email` carries its IMAP UID.
- `MarkSeen(ctx, settings, uidSet) error` — the existing `Store` call, now invoked independently (in its own, later IMAP session) only after the DB transaction below has committed, using `Store` with a `UIDSet` (issuing `UID STORE`) instead of a `SeqSet`.

This UID switch is required, not optional, because the two steps now run in separate IMAP sessions: message *sequence numbers* are only stable within a single session and are renumbered if any message is expunged between sessions (by this process or a concurrent IMAP client) — marking by stale sequence numbers in a later session could silently mark the wrong messages `\Seen`, reintroducing the exact data-loss failure this spec exists to prevent. UIDs are stable identifiers for a message within a mailbox's `UIDVALIDITY` epoch, and `go-imap/v2`'s `Client.Fetch`/`Client.Store` both accept the same `imap.NumSet` interface satisfied by either `SeqSet` or `UIDSet` — passing a `UIDSet` is a drop-in call-site change (it automatically issues `UID FETCH`/`UID STORE`), not a library upgrade or rewrite. (Mailbox `UIDVALIDITY` changes — a rare, whole-mailbox event — are not handled specially here; that's an accepted, documented gap, not a silent one.)

**New `domain/mail` package**, mirroring `domain/setting`'s existing `model.go` / `ports.go` / `service.go` / `errors.go` structure:
- `Repository.SaveFetchedMails(ctx, mails []NewMailWithEvent) ([]SaveResult, error)` is the one method that matters. It opens a single Postgres transaction and, for each mail: inserts into `mails` with `ON CONFLICT (setting_id, message_id) DO NOTHING`; only if that insert actually happened, inserts the corresponding `outbox` row (topic `email-received`, key and payload as described above). Commits once for the whole batch.
- The event payload is built *before* the repository call, at the domain/service layer, using the existing `EmailCreatedPayload` struct from `libs/events` — the repository only ever handles raw bytes, it never constructs or interprets the payload.
- The method reports, per mail, whether it's now durably accounted for (true whether newly inserted or already existing from a prior crash) — which in practice is every mail in the batch, since a conflict just means "already recorded, safe to consider handled."

**`email_service.go`'s `createTask`** changes to: fetch unseen mails → call `mailService.SaveFetchedMails(...)` for the batch → if that succeeds, call `imapService.MarkSeen(...)` for the *entire* originally-fetched sequence set → done. If the transaction fails, log and return without marking anything seen — the same batch is safely retried on the next poll (the unique constraint makes this idempotent).

## `libs/outbox` (reusable relay library)

```go
package outbox

type Record struct {
    ID      int64
    Topic   string
    Key     []byte
    Payload []byte
}

// Store is implemented per-service against its own outbox table.
// This is the customization point for future services — each service's
// Postgres schema and business logic stay entirely its own.
type Store interface {
    FetchBatch(ctx context.Context, limit int) ([]Record, error) // WHERE processed_at IS NULL AND failed_at IS NULL, FOR UPDATE SKIP LOCKED, ORDER BY id
    MarkProcessed(ctx context.Context, id int64) error
    MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error // increments attempts, sets last_error; sets failed_at once attempts >= maxAttempts
}

type Relay struct { /* store Store; client *kgo.Client; pollInterval time.Duration; batchSize, maxAttempts int */ }

func NewRelay(store Store, client *kgo.Client, opts ...Option) *Relay
func (r *Relay) Run(ctx context.Context) error // blocking poll loop
```

`libs/outbox` also exports `NewProducerClient(brokers []string) (*kgo.Client, error)`, wrapping `kgo.NewClient(kgo.SeedBrokers(...), kgo.AllowAutoTopicCreation())` — baking in the auto-topic-creation lesson learned during the NATS-to-Kafka migration (franz-go disables broker-side auto-topic-creation client-side by default) so every future sidecar built on this library gets it for free instead of rediscovering that bug.

`FOR UPDATE SKIP LOCKED` in `FetchBatch` is a deliberate concurrency-safety choice: it costs nothing when running a single relay replica (the current plan) and makes horizontal scaling of the sidecar safe later without any retrofit — two replicas polling concurrently will never claim the same row.

"Custom processing logic" lives entirely in each service's `Store` implementation, never in `Relay` itself, which only ever moves opaque bytes — confirmed as the intended extension point: `Relay` has no pre-produce hook, since topic/key/payload are already fully decided at write time by the service that owns the data. This is what keeps the engine reusable across services with completely different table shapes and message formats, and keeps schema-registry serialization firmly the writing service's responsibility.

## Sidecar Deployment

- **Binary:** `services/email/cmd/outbox-relay/main.go` — a separate `main` package, independent of the email service's own `cmd/main.go`, but living in the same module tree since it's tightly coupled to email's Postgres schema.
- **Config:** its own minimal config struct (not the full `services/email/internal/config.ApplicationConfiguration`, which would force unrelated `KC_*` Keycloak env vars to be set just to run a process that never touches Keycloak or REST): `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` (same names/values the email service already uses), `KAFKA_BROKERS` (same name used by core/email), `OUTBOX_POLL_INTERVAL` (default `1s`), `OUTBOX_BATCH_SIZE` (default `100`), `OUTBOX_MAX_ATTEMPTS` (default `5`).
- **Dockerfile:** `services/email/Dockerfile.outbox-relay` — at the root of the `services/email` folder (not nested under `cmd/`), a standard multi-stage Go build, built from the monorepo root as context (`docker build -f services/email/Dockerfile.outbox-relay .`), matching the CI convention already referenced by `Taskfile.yml`/`.github/workflows/`. The main service's own `services/email/Dockerfile` and `services/core/Dockerfile` do not exist in the repo today despite being referenced by the Taskfile and CI — a pre-existing gap, to be filled in a separate, future piece of work, not part of this spec.
- **docker-compose:** a new `email-outbox-relay` service in `dev/docker-compose.yaml`, `depends_on: [postgres, kafka]`, env vars matching the local dev values already used by `postgres`/`kafka` in that file.
- **Taskfile:** `run:email:outbox-relay` (Air hot-reload, mirroring `run:core`/`run:email`) and `build:email:outbox-relay` (docker build, mirroring `build:core`/`build:email`).

## Error Handling & Retry Semantics

- **Write path:** DB transaction failure → nothing committed, nothing marked seen, whole batch safely retried next poll (idempotent via the unique constraint). IMAP `MarkSeen` failure *after* a successful commit → mails/outbox rows already durably exist, so re-fetched duplicates are harmless no-ops; only the seen-marking is retried.
- **Relay:** Kafka produce failure → `attempts` increments, `last_error` set, row stays unprocessed and is retried next poll, until `attempts >= OUTBOX_MAX_ATTEMPTS`, at which point `failed_at` is set (poison-row quarantine — excluded from further polling, retained for inspection).
- **Consumer side (`services/core` subscribing to `email-received`):** unchanged by this work; out of scope here.

## Testing

- **`domain/mail` service:** unit tests with a mocked `Repository`, following the exact pattern of `domain/setting/service_test.go`.
- **`services/email/infra/postgres` outbox+mail repository:** integration test using the existing testcontainer setup, verifying the atomic dual-insert and the `ON CONFLICT DO NOTHING` dedup behavior under a simulated duplicate fetch.
- **`libs/outbox` Relay:** unit tests against a fake in-memory `Store` (no real DB/Kafka needed) — verifies poll → produce → mark-processed / mark-failed → quarantine transitions purely through the interface.
- **Sidecar binary itself:** no additional tests beyond what's already covered by the library and repository tests — `main.go` is wiring only.

## Explicitly Deferred (Future Work)

- Kafka-side dead-letter topic for `services/core`'s subscribe path (unrelated to this spec, already noted in the NATS-to-Kafka migration spec).
- Message headers / trace-context propagation on outbox rows. When this is picked up: add a `headers JSONB` column to `outbox` (nullable, additive migration — existing rows just have `NULL`), populated at write time by the email service with a serialized W3C Trace Context (`traceparent`/`tracestate`) captured from whatever span represents "this email became this outbox event." `headers` is deliberately separate from `payload`: headers are always plain ASCII strings, so unlike the payload they carry no binary/schema-registry constraint, and a consumer that doesn't care about tracing can ignore them without needing to understand the event schema. The trace context must be persisted in the row (not just held in-memory) because the relay is a separate process that reads the row later — a `context.Context`'s trace info doesn't survive past the goroutine that created it. At send time, `libs/outbox`'s `Relay` would translate the stored `headers` into native `kgo.Record.Headers` on the produced record; on the consuming side (`services/core`), the same headers would be read back out to continue the trace into request creation/classification.
- Outbox table retention/cleanup job (processed rows accumulate indefinitely for now).
- Schema-registry integration (Avro/Protobuf serialization) — the `bytea` payload column is what makes this addable later without a further migration, but the registry work itself is not part of this spec.
- Whether schema-registry serialization logic, once added, belongs in the writing service or in a shared library — an open question the team flagged for a later decision, not resolved here.
- Reuse of `libs/outbox` by any service other than `services/email` — the library is designed for reuse, but no second consumer is being built in this pass.
- The pre-existing missing `services/core/Dockerfile` and `services/email/Dockerfile` — unrelated gap, separate future work.
