# Kafka Schema Registry + Avro (v1)

## Purpose

`services/email` and `services/core` currently exchange the `email.received` event as raw JSON, with the wire shape (`contracts.EmailCreatedPayload`) hand-maintained as a Go struct with no schema enforcement, no compatibility checking, and no protection against producer/consumer drift. This spec introduces Avro + a schema registry (Apicurio) as the serialization mechanism for Kafka messages, replacing hand-maintained JSON types with schema-generated ones, and fixes a pre-existing hexagonal-architecture violation where both `domain/mail` and `domain/inbox` perform serialization directly instead of leaving it to the infra layer.

This is the phase explicitly deferred by `2026-07-09-nats-to-kafka-migration-design.md` ("Explicitly deferred: Schema registry, Avro/Protobuf...").

## Goals

- Replace hand-maintained JSON payload types with Avro schemas (`.avsc`) as the single source of truth, with Go types generated from them.
- Run a schema registry locally (Apicurio, via docker-compose) with compatibility enforcement.
- Establish conventions (directory layout, codegen command, registration flow, subject naming) generic enough that onboarding a second event type is "add a `.avsc` file," not "redesign the pattern."
- Fix the domain-layer serialization violation in `domain/mail` and `domain/inbox` as part of this work, since every call site touching serialization is being rewritten anyway.

## Non-goals

- Splitting `email`/`core`/`contracts` into separate git repos, or publishing `contracts` as an external Go module. The design keeps the door open for this (see Section 5) but does not build it.
- CI schema-linting, multi-registry governance, schema-drift alerting.
- Alternate subject-naming strategies beyond TopicNameStrategy.
- Production/staging registry provisioning (would go through OpenTofu, like the existing Kafka topics/credentials — separate future work).
- Protobuf/JSON-Schema support (Apicurio supports both, but only Avro is in scope here).
- Any event type other than `email.received`.

## 1. Registry: Apicurio, local dev infra

New service in `dev/docker-compose.yaml`:

```yaml
schema-registry:
  image: apicurio/apicurio-registry:3.3.0
  container_name: schema-registry
  profiles: ["kafka-clients"]
  ports:
    - "8083:8080"
```

No `APICURIO_STORAGE_KIND` is set, so it defaults to in-memory storage. This is a deliberate simplification: `task down` already wipes all Docker volumes, so a Postgres-backed registry wouldn't survive your normal reset workflow either — in-memory avoids a new database, a new `docker-entrypoint-initdb.d` script, and a coupling to the `postgres` container, for no durability benefit under this repo's dev workflow.

Port `8083` is used because 8080/8081/8082 are already taken by Keycloak, pgadmin, and kafka-ui respectively. The service is gated behind the existing `kafka-clients` Compose profile, matching the producer/consumer sidecars — it's only relevant when those run.

Apicurio exposes a Confluent-compatible REST API at `/apis/ccompat/v7` on the same port. This is the endpoint both `franz-go/pkg/sr` and the registration CLI (Section 4) target: `http://localhost:8083/apis/ccompat/v7` from the host, `http://schema-registry:8080/apis/ccompat/v7` from other containers.

**Compatibility mode: BACKWARD**, set as the registry's default for the `email.received-value` subject. A new schema version must be readable using the previous schema — in practice, new fields must declare an Avro `default`. Operational consequence: consumers get deployed before producers when rolling out a compatible schema change, since BACKWARD guarantees new-reader-reads-old-data but not the reverse.

## 2. Libraries

- **`github.com/iskorotkov/avro`** (actively maintained fork of `hamba/avro`, which was archived January 2026) — the Avro codec, plus its `avrogen` code-generation tool. Chosen over `linkedin/goavro` because goavro only supports generic `map[string]interface{}` values with no struct-tag binding and no codegen — using it would mean writing manual, unchecked-until-runtime field access/construction code on both produce and consume, a regression from today's typed `json.Unmarshal` into `EmailCreatedPayload`.
- **`github.com/twmb/franz-go/pkg/sr`** — the schema registry HTTP client and Confluent wire-format envelope (magic byte + 4-byte schema ID), used via `sr.Serde` to compose with the Avro codec. This is a separate concern from the codec: `pkg/sr` doesn't know how to serialize Avro, it only manages registry communication, ID caching, and the wire envelope; you register your own encode/decode functions (backed by `iskorotkov/avro`) against a schema ID with `Serde.Register`.
- Wire format: the Confluent standard envelope (magic byte `0x0` + 4-byte schema ID + Avro payload), since that's what `pkg/sr` and registry-aware tooling expect natively.
- Subject naming: **TopicNameStrategy** — `email.received-value`.

## 3. Schema ownership: `libs/events/contracts`

`libs/events/contracts` (currently holding the hand-written `EmailCreatedPayload` struct and `EmailReceivedTopic` constant) becomes the schema-owning package:

```
libs/events/contracts/
├── schemas/
│   └── email_created.avsc          # source of truth, hand-edited
├── email_created_gen.go             # generated by avrogen, checked in
├── contracts.go                     # EmailReceivedTopic, Schemas(), serde helpers
```

`email_created.avsc`:

```json
{
  "type": "record", "name": "EmailCreated", "namespace": "planeo.email",
  "fields": [
    {"name": "subject", "type": "string"},
    {"name": "body", "type": "string"},
    {"name": "from", "type": "string"},
    {"name": "date", "type": {"type": "long", "logicalType": "timestamp-millis"}},
    {"name": "messageId", "type": "string"},
    {"name": "organizationId", "type": "int"}
  ]
}
```

`.avsc` files are embedded into the package via `go:embed` and exposed through a small public function, e.g. `contracts.Schemas() map[string]avro.Schema` keyed by subject name, so nothing outside the package reaches for raw file paths.

Codegen command (wired as `task generate:avro`, mirroring the existing `task web:generate-client` pattern):

```
avrogen -pkg contracts -o email_created_gen.go -tags json:camel libs/events/contracts/schemas/email_created.avsc
```

The `-tags json:camel` flag makes `avrogen` emit **both** `avro:` and `json:` struct tags on the same generated type. This one struct serves two purposes: Avro wire encoding (Section 6) and JSON staging in the outbox (Section 7) — there is no second, independently-maintained struct that could drift out of sync with the schema. (Note: this flag is confirmed for the original `hamba/avro`'s `avrogen`; the `iskorotkov/avro` fork is described as drop-in compatible, but the flag should be verified against the fork's actual CLI during implementation.)

```go
// email_created_gen.go (generated)
type EmailCreated struct {
    Subject        string    `avro:"subject" json:"subject"`
    Body           string    `avro:"body" json:"body"`
    From           string    `avro:"from" json:"from"`
    Date           time.Time `avro:"date" json:"date"`
    MessageId      string    `avro:"messageId" json:"messageId"`
    OrganizationId int32     `avro:"organizationId" json:"organizationId"`
}
```

`contracts.go` adds thin serde helpers wrapping `sr.Serde`, e.g. `EncodeEmailCreated(v EmailCreated) ([]byte, error)` and `DecodeEmailCreated(b []byte, v *EmailCreated) error`, so callers never touch `sr.Serde` or the registry client directly.

`contracts` continues to import nothing from `email` or `core`. This is deliberate boundary discipline: since the repo is expected to split into separate repos eventually, keeping `contracts` fully self-contained (only stdlib + the Avro codec + `pkg/sr`) means a future extraction is a matter of moving the directory and giving it its own `go.mod` — not a redesign. Nothing in this spec builds that extraction; the single-module structure (`CLAUDE.md`: "Single `go.mod` at the root") is unchanged.

## 4. Schema registration

Schemas are registered via a **content-based lookup-or-register** call — the registry is asked "what ID matches this exact schema content," which returns the existing ID if it's already registered rather than creating a duplicate. This is never done via producer auto-registration at runtime; auto-registration would let a running process silently publish an unreviewed schema change.

New Go command, `libs/events/contracts/cmd/registerschemas/main.go`, using `pkg/sr`'s registration calls, walking `contracts.Schemas()` and registering each one against the configured registry URL. Wired as a new Task target, `task register:schemas`, called from `task up` immediately after the existing migration step — same "safe to rerun every time" property as goose migrations: registering an already-identical schema is a no-op that returns the existing ID.

**Producers always resolve their schema ID by matching their own compiled-in schema content, never by fetching "latest."** This is what guarantees a producer's wire output is fully determined by its deployed code: if a producer is still running `EmailCreated` v1, it always encodes and stamps v1's ID, regardless of whether v2 has since been registered elsewhere. (If this rule were violated — e.g. a producer fetched "latest" while still running v1-generated code — `avro.Marshal` would either silently fill any new field's declared default, producing messages that look like v2 but never carry real data in that field, or error outright if the new field lacked a default. Pinning to compiled-in content avoids this failure mode entirely.)

## 5. Runtime flow

**Produce** (first message of a process's lifetime for a given schema version):
1. Look up the schema ID matching the process's compiled-in `EmailCreated` schema (real HTTP call to Apicurio).
2. `sr.Serde` caches that ID for the process's lifetime.
3. Every message after that: `avro.Marshal` + prepend the cached ID + magic byte — pure local computation, no per-message network call.

**Consume**:
1. Read the magic byte + 4-byte schema ID from the message header.
2. If this ID hasn't been seen before, fetch the writer schema from the registry (cached afterward).
3. Avro's schema resolution walks the bytes according to the writer schema (the only schema that correctly describes this exact byte layout) while producing a value shaped by the reader schema (the consumer's own compiled-in `EmailCreated`) — filling in the reader's declared defaults for fields the writer didn't have, and dropping fields the writer had that the reader doesn't.

## 6. Hexagonal boundary fix: `domain/mail` (producer side)

Today, `domain/mail/service.go`'s `buildMailAndEvent` constructs `contracts.EmailCreatedPayload` and calls `json.Marshal` directly — `domain/mail` importing `contracts` for this purpose was accepted as a stopgap in `2026-07-12-outbox-architecture-cleanup-design.md` (see its Section 1 forward-looking note). This spec resolves it: `domain/mail` stops importing `contracts` entirely.

`domain/mail/model.go` gains a plain domain struct and `OutboxEvent` changes shape:

```go
type EmailReceivedFields struct {
    Subject        string
    Body           string
    From           string
    Date           time.Time
    MessageID      string
    OrganizationID int
}

type OutboxEvent struct {
    Key    []byte
    Fields EmailReceivedFields
}
```

`buildMailAndEvent` builds `OutboxEvent{Key: []byte(strconv.Itoa(raw.OrganizationID)), Fields: EmailReceivedFields{...}}` — no `contracts` import, no marshaling.

The mapping from `EmailReceivedFields` into the generated wire struct, and the topic name, move into the infra repository, which already imports `contracts`:

```go
// infra/postgres/outbox_repository.go
func (c *Client) CreateOutboxEvent(ctx context.Context, mailID int, event mail.OutboxEvent) error {
    wire := contracts.EmailCreated{
        Subject:        event.Fields.Subject,
        Body:           event.Fields.Body,
        From:           event.Fields.From,
        Date:           event.Fields.Date,
        MessageId:      event.Fields.MessageID,
        OrganizationId: int32(event.Fields.OrganizationID),
    }
    payload, err := json.Marshal(wire) // JSON, not Avro — see Section 7
    if err != nil {
        return NewDatabaseError("error marshaling outbox payload", err)
    }
    query := `INSERT INTO outbox (mail_id, topic, key, payload) VALUES (@mailId, @topic, @key, @payload)`
    args := pgx.NamedArgs{
        "mailId":  mailID,
        "topic":   contracts.EmailReceivedTopic,
        "key":     event.Key,
        "payload": payload,
    }
    // ... unchanged exec logic
}
```

## 7. Outbox: `payload` column becomes `JSONB`

The outbox write happens inside the same transaction as the `mails` insert (see `2026-07-12-outbox-architecture-cleanup-design.md` Section 4) — well before the message is actually produced to Kafka. Avro-encoding (and its registry-ID lookup) at that point would put the registry dependency inside the domain's transactional write path, and would freeze a schema ID into the row the moment it's written, before it's even been sent.

Instead, the outbox row stores the **JSON-marshaled generated struct** (Section 6), and a new migration changes `outbox.payload` from `BYTEA` to `JSONB`:

```sql
-- new goose migration, e.g. services/email/internal/infra/postgres/migrations/<timestamp>_outbox_payload_jsonb.sql
ALTER TABLE outbox ALTER COLUMN payload TYPE JSONB USING convert_from(payload, 'UTF8')::jsonb;
```

The Avro encode + registry-ID lookup happens later, in the `email-received-producer` sidecar (`services/email/cmd/email-received-producer/`), immediately before the bytes reach Kafka.

This has to be placed carefully. `domain/outbox` (the relay sidecar's own domain package, distinct from `domain/mail`) already defines `Service.ProcessRecord(ctx, rec libsoutbox.Record, maxAttempts int) error`, which today forwards `rec.Payload` straight to `Producer.ProduceSync(ctx, topic string, key, value []byte) error` unchanged — that's correct today because the outbox payload already is the final wire bytes. Putting the JSON-unmarshal-then-Avro-encode step inside `ProcessRecord` would make `domain/outbox` import `contracts`, reintroducing the same violation this spec fixes elsewhere. `ProcessRecord` is left untouched; instead, a new infra adapter wraps the `Producer` port so the conversion happens beneath the domain layer, invisibly to it:

```go
// infra/events/email_created_producer.go — new
type EmailCreatedProducer struct {
    inner outbox.Producer // the real franz-go-backed producer (libsoutbox.NewProducer's return value)
}

func (p *EmailCreatedProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
    var wire contracts.EmailCreated
    if err := json.Unmarshal(value, &wire); err != nil {
        return fmt.Errorf("decode staged outbox payload: %w", err)
    }
    avroBytes, err := contracts.EncodeEmailCreated(wire) // avro.Marshal + registry ID lookup, cached
    if err != nil {
        return fmt.Errorf("avro encode: %w", err)
    }
    return p.inner.ProduceSync(ctx, topic, key, avroBytes)
}
```

`cmd/main.go` wires `EmailCreatedProducer{inner: libsoutbox.NewProducer(...)}` in as the `outbox.Producer` implementation passed into `outbox.NewService(...)`, in place of the raw producer used today. `domain/outbox.Service.ProcessRecord` requires no code change at all — it keeps calling `s.producer.ProduceSync(ctx, rec.Topic, rec.Key, rec.Payload)` exactly as it does now, unaware that the bytes it passes get converted from JSON to Avro underneath.

This keeps the registry dependency entirely out of the transactional write path, and keeps pending outbox rows human-readable via plain SQL (`SELECT payload FROM outbox WHERE status = 'pending'`) instead of opaque binary — useful for debugging stuck rows.

`libs/outbox`'s `Record`/`Store` types (`Payload []byte`) are unaffected structurally — Postgres returns `JSONB` as bytes over the wire either way; only the column type and what's inside the bytes changes (JSON text instead of pre-Avro-encoded binary).

## 8. Hexagonal boundary fix: `domain/inbox` (consumer side)

The inbox table's `payload BYTEA` column is unchanged — it has to keep storing raw wire bytes (Avro-encoded, Confluent-enveloped) as received from Kafka, since durability must happen before any decode attempt (which requires a registry round-trip) is even possible. Only the *decode call site* moves.

Today, `domain/inbox/service.go`'s `ProcessEmailReceived(ctx, rec libsinbox.Record, maxAttempts int)` does `json.Unmarshal(rec.Payload, &payload)` itself. This spec moves the decode into `infra/inbox/email_received_consumer.go` — `contracts` is a new import for that adapter, which doesn't need it today:

```go
// domain/inbox/model.go — new
type EmailReceivedEvent struct {
    Subject        string
    Body           string
    From           string
    Date           time.Time
    MessageID      string
    OrganizationID int
}
```

```go
// domain/inbox/ports.go — Service interface
ProcessEmailReceived(ctx context.Context, recordID int64, event EmailReceivedEvent, maxAttempts int) error
MarkDecodeFailure(ctx context.Context, recordID int64, err error, maxAttempts int) error // new: exposes the existing MarkFailed repository call for infra to use when decoding fails before domain logic ever runs
```

```go
// infra/inbox/email_received_consumer.go
func (a *EmailReceivedConsumer) PollOnce(ctx context.Context) error {
    records, err := a.service.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
    if err != nil {
        return err
    }
    for _, rec := range records {
        var wire contracts.EmailCreated
        if err := contracts.DecodeEmailCreated(rec.Payload, &wire); err != nil {
            if err := a.service.MarkDecodeFailure(ctx, rec.ID, err, a.maxAttempts); err != nil {
                return err
            }
            continue
        }
        event := inbox.EmailReceivedEvent{
            Subject: wire.Subject, Body: wire.Body, From: wire.From,
            Date: wire.Date, MessageID: wire.MessageId, OrganizationID: int(wire.OrganizationId),
        }
        if err := a.service.ProcessEmailReceived(ctx, rec.ID, event, a.maxAttempts); err != nil {
            return err
        }
    }
    return nil
}
```

`domain/inbox/service.go`'s `ProcessEmailReceived` drops its `contracts` import and `json.Unmarshal` call entirely — it receives an already-typed `EmailReceivedEvent` and proceeds directly to the existing LLM-extraction/classification and `Request` upsert logic, unchanged.

## Testing implications

- `domain/mail/service_test.go`: `buildMailAndEvent`'s test assertions change from checking JSON-marshaled bytes to checking the plain `EmailReceivedFields` struct — no `contracts` import needed in this test file anymore.
- `services/email/internal/test/mail/mail_test.go` (integration): asserts the outbox row's `payload` column is valid JSON matching `contracts.EmailCreated`'s shape (via `json.Unmarshal` in the test), not raw bytes.
- New integration test for `email-received-producer`: given an outbox row with JSON payload, verify it Avro-encodes and calls `ProduceSync` with a correctly-enveloped payload (magic byte + registered schema ID + Avro bytes) — requires the schema-registry testcontainer (new addition to `internal/test/utils/`).
- `domain/inbox/service_test.go`: `ProcessEmailReceived`'s mock `Repository`/`Service` no longer needs to exercise unmarshal-failure paths (those move to the new `MarkDecodeFailure` path, tested at the infra adapter level instead).
- New test for `infra/inbox/email_received_consumer.go`: decode failure (malformed/wrong-schema bytes) calls `MarkDecodeFailure` and continues to the next record rather than aborting the batch; decode success maps all six fields correctly into `EmailReceivedEvent`.
- `libs/events/contracts`: new unit tests for `EncodeEmailCreated`/`DecodeEmailCreated` round-tripping, and for `Schemas()` returning a schema that matches the embedded `.avsc` file content.
- Registration CLI (`cmd/registerschemas`): integration test against the Apicurio testcontainer verifying idempotent re-registration (same content twice returns the same ID, no error).

## Explicitly deferred / out of scope

- Publishing `libs/events/contracts` as an external Go module; per-repo CI; separate dev-environment repo for local infra.
- Multi-schema/multi-topic governance tooling (schema linting in CI, alternate subject-naming strategies).
- Schema-drift alerting/notification tooling.
- Production/staging registry provisioning via OpenTofu.
- Protobuf/JSON-Schema support in Apicurio (registry supports it; not used here).
- Retrofitting any event type other than `email.received`.
