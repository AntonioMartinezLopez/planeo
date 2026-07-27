# Kafka Schema Registry + Avro (v1)

## Purpose

`services/email` and `services/core` currently exchange the `email.received` event as raw JSON, with the wire shape (`contracts.EmailCreatedPayload`) hand-maintained as a Go struct with no schema enforcement, no compatibility checking, and no protection against producer/consumer drift. This spec introduces Avro + a schema registry (Apicurio) as the serialization mechanism for Kafka messages, replacing hand-maintained JSON types with schema-generated ones, and fixes a pre-existing hexagonal-architecture violation where both `domain/mail` and `domain/inbox` perform serialization directly instead of leaving it to the infra layer.

This is the phase explicitly deferred by `2026-07-09-nats-to-kafka-migration-design.md` ("Explicitly deferred: Schema registry, Avro/Protobuf...").

## Goals

- Replace hand-maintained JSON payload types with Avro schemas (`.avsc`) as the single source of truth, with Go types generated from them.
- Run a schema registry locally (Apicurio, via docker-compose) with compatibility enforcement.
- Establish conventions (directory layout, codegen command, registration flow, subject naming) generic enough that onboarding a second event type is "add a `.avsc` file," not "redesign the pattern."
- Fix the domain-layer serialization violation in `domain/mail` and `domain/inbox` as part of this work, since every call site touching serialization is being rewritten anyway.
- Keep the generic Kafka/registry/Avro machinery and the app-specific event schemas as two separately-extractable packages, since they change at different rates and have different reuse scope (see Section 3).

## Non-goals

- Splitting `email`/`core`/`libs/schemaregistry`/`libs/eventschemas` into separate git repos, or publishing either as an external Go module. The design keeps the door open for this (Section 3) but does not build it.
- CI schema-linting, multi-registry governance, schema-drift alerting.
- Alternate subject-naming strategies beyond TopicNameStrategy.
- Production/staging registry provisioning (would go through OpenTofu, like the existing Kafka topics/credentials — separate future work), including which registry auth scheme to use there.
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

**Known limitation of this choice:** because the registry is wiped on every restart, `task register:schemas` on a fresh `task up` only ever sees whatever `.avsc` content is currently checked out — it has no memory of any prior version, so it registers the current schema as "version 1" every time. This means the registry **cannot catch a compatibility violation during normal local dev** — compatibility checking works by comparing against the previous version, and after a wipe there is no previous version to compare against, so any schema registers cleanly regardless of whether it would actually break BACKWARD compatibility against what's really running elsewhere. Real compatibility enforcement only becomes meaningful once a registry persists across changes — i.e. a shared/staging registry, which is deferred (see Non-goals). Locally, this can still be exercised deliberately within a single continuous session (register v1, produce with it, evolve to v2, register again without restarting the registry in between) — this is exactly what the schema-evolution round-trip test in Testing implications does — but it does not happen implicitly through everyday `task up`/`task down` cycles.

Port `8083` is used because 8080/8081/8082 are already taken by Keycloak, pgadmin, and kafka-ui respectively. The service is gated behind the existing `kafka-clients` Compose profile, matching the producer/consumer sidecars — it's only relevant when those run.

Apicurio exposes a Confluent-compatible REST API at `/apis/ccompat/v7` on the same port. This is the endpoint `libs/schemaregistry` (Section 3) and the registration CLI (Section 4) target: `http://localhost:8083/apis/ccompat/v7` from the host, `http://schema-registry:8080/apis/ccompat/v7` from other containers.

**Compatibility mode: BACKWARD**, set as the registry's default for the `email.received-value` subject. A new schema version must be readable using the previous schema — in practice, new fields must declare an Avro `default`. Operational consequence: consumers get deployed before producers when rolling out a compatible schema change, since BACKWARD guarantees new-reader-reads-old-data but not the reverse.

## 2. Libraries

- **`github.com/iskorotkov/avro`** (actively maintained fork of `hamba/avro`, which was archived January 2026) — the Avro codec, plus its `avrogen` code-generation tool. Chosen over `linkedin/goavro` because goavro only supports generic `map[string]interface{}` values with no struct-tag binding and no codegen — using it would mean writing manual, unchecked-until-runtime field access/construction code on both produce and consume, a regression from today's typed `json.Unmarshal` into `EmailCreatedPayload`.
- **`github.com/twmb/franz-go/pkg/sr`** — used at its lower level (`sr.Client`'s `LookupSchema`/`CreateSchema`/`RegisterSchema`/`SchemaByID` methods, and the exported `ConfluentHeader` for the wire envelope), **not** via its `Serde` convenience type. `Serde` requires registering an encode/decode function per concrete Go type, which is exactly the kind of app-specific coupling Section 3 keeps out of the generic layer. Building on the lower-level client instead means `libs/schemaregistry`'s public API can be expressed with Go generics (`Encode[T any]`/`Decode[T any]`) and stays fully generic. The trade-off, made deliberately: we own the schema-ID caching and resolution correctness that `Serde` would otherwise provide — see Section 3's `Client` sketch for exactly what that entails.
- Wire format: the Confluent standard envelope (magic byte `0x0` + 4-byte schema ID + Avro payload), via `sr.ConfluentHeader`, since that's what registry-aware tooling expects natively.
- Subject naming: **TopicNameStrategy** — `email.received-value`.

## 3. Two packages: `libs/schemaregistry` (generic) and `libs/eventschemas` (app-specific)

These are kept as two separate packages rather than one, because they change at different rates and have different reuse scope: `libs/schemaregistry` has zero knowledge of what an "email" is and rarely changes; `libs/eventschemas` changes every time an event is added or evolved. Bundling them would mean every schema change also version-bumps "how we talk to the registry," and would mean a future extraction of the generic engine (a legitimate standalone "Kafka + Avro + schema registry" toolkit, useful outside this app) drags planeo's specific event types along with it. Nothing in this spec builds that extraction — both packages just stay self-contained enough that it stays cheap later (only stdlib + `iskorotkov/avro` + `pkg/sr`; no imports from `email`, `core`, or each other beyond `eventschemas`'s own use of the Avro codec types).

### `libs/schemaregistry` — generic engine

```go
// libs/schemaregistry/client.go
type Client struct {
    sr          *sr.Client
    header      sr.ConfluentHeader
    idBySubject sync.Map // subject string -> int, resolved once via content-based lookup
    schemaByID  sync.Map // int -> avro.Schema, fetched lazily on first decode of that ID
}

func NewClient(opts ...sr.ClientOpt) (*Client, error) { /* wraps sr.NewClient(opts...) */ }

// Encode resolves subject's registry ID (content-based lookup, cached), marshals
// v against schema, and returns the Confluent-enveloped bytes.
func Encode[T any](ctx context.Context, c *Client, subject string, schema avro.Schema, v T) ([]byte, error) {
    id, err := c.resolveID(ctx, subject, schema) // sr.Client.LookupSchema, cached in idBySubject
    if err != nil {
        return nil, err
    }
    body, err := avro.Marshal(schema, v)
    if err != nil {
        return nil, err
    }
    return c.header.AppendEncode(nil, id, nil), body // exact append shape TBD at implementation
}

// Decode reads the schema ID from b's envelope, fetches (and caches) that writer
// schema, resolves it against readerSchema, and unmarshals into v.
//
// readerSchema must be passed explicitly: avro.Unmarshal(schema, data, v) takes
// only ONE schema, and that one is the writer schema — passing just the writer
// schema does NOT automatically apply readerSchema's declared defaults for fields
// the writer didn't have. iskorotkov/avro (inherited from hamba/avro) exposes
// SchemaCompatibility.Resolve(reader, writer Schema) (Schema, error) for this;
// the exact call sequence (whether Resolve's return value is what gets passed to
// Unmarshal, or something else) needs confirming against the real library before
// this is implemented — flagged here rather than guessed.
func Decode[T any](ctx context.Context, c *Client, b []byte, readerSchema avro.Schema, v *T) error {
    id, body, err := c.header.DecodeID(b)
    if err != nil {
        return err
    }
    writerSchema, err := c.writerSchema(ctx, id) // sr.Client.SchemaByID, cached in schemaByID
    if err != nil {
        return err
    }
    resolved, err := avro.NewSchemaCompatibility().Resolve(readerSchema, writerSchema)
    if err != nil {
        return err
    }
    return avro.Unmarshal(resolved, body, v)
}
```

Each service adds a `SCHEMA_REGISTRY_URL` env var to its `internal/config/config.go` (same pattern as `NATS_URL`, `DB_*`, `KC_*`), and `cmd/main.go` constructs the client once — `schemaregistry.NewClient(sr.URLs(cfg.SchemaRegistryURL))` for local dev, with `sr.BasicAuth(...)`/`sr.BearerToken(...)`/`sr.DialTLSConfig(...)` added once a non-local registry's auth scheme is decided (out of scope; see Non-goals) — then injects it into the infra adapters that need it. This mirrors how every other external client in this codebase is wired, and means moving from the local registry to a remote one is a config change, not a code change.

### `libs/eventschemas` — schemas and generated types

```
libs/eventschemas/
├── schemas/
│   └── email_created.avsc          # source of truth, hand-edited
├── email_created_gen.go             # generated by avrogen, checked in
├── eventschemas.go                  # EmailReceivedTopic, EmailReceivedSubject, Schemas()
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

`.avsc` files are embedded via `go:embed` and exposed through `eventschemas.Schemas() map[string]avro.Schema`, keyed by subject name, so nothing outside the package reaches for raw file paths — this is what the registration CLI (Section 4) walks generically.

Codegen command (wired as `task generate:avro`, mirroring the existing `task web:generate-client` pattern):

```
avrogen -pkg eventschemas -o email_created_gen.go -tags json:camel libs/eventschemas/schemas/email_created.avsc
```

The `-tags json:camel` flag makes `avrogen` emit **both** `avro:` and `json:` struct tags on the same generated type — one struct serves both Avro wire encoding and JSON staging in the outbox (Section 7). (Confirmed for the original `hamba/avro`'s `avrogen`; `iskorotkov/avro` is described as drop-in compatible, but the flag should be verified against the fork's actual CLI during implementation.)

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

`eventschemas` has **no** dependency on `libs/schemaregistry` and defines no encode/decode functions of its own — it only holds data (schemas, generated structs, the topic/subject name constants). This is what makes it a plain, replaceable dependency: onboarding a second event type touches only this package, never `libs/schemaregistry`.

### Where the coupling lives: infra adapters

Neither package couples a specific type to the generic engine — that happens explicitly in the infra adapters that need it (Sections 7 and 8), which import both `libs/schemaregistry` and `libs/eventschemas` and call `schemaregistry.Encode[eventschemas.EmailCreated](ctx, client, eventschemas.EmailReceivedSubject, eventschemas.EmailCreatedSchema, wire)` directly. This is deliberate: an adapter's job in hexagonal architecture is exactly to wire concrete things together, so this is where "which type, which subject, which schema" belongs — not hidden behind a per-type wrapper function, and not pushed into either generic package.

## 4. Schema registration

Schemas are registered via a **content-based lookup-or-register** call — the registry is asked "what ID matches this exact schema content," which returns the existing ID if it's already registered rather than creating a duplicate. This is never done via producer auto-registration at runtime; auto-registration would let a running process silently publish an unreviewed schema change.

New Go command, `libs/eventschemas/cmd/registerschemas/main.go`, using `libs/schemaregistry`'s client plus `pkg/sr`'s registration calls, walking `eventschemas.Schemas()` and registering each one against the configured registry URL. Wired as a new Task target, `task register:schemas`, called from `task up` immediately after the existing migration step — same "safe to rerun every time" property as goose migrations: registering an already-identical schema is a no-op that returns the existing ID.

**Producers always resolve their schema ID by matching their own compiled-in schema content, never by fetching "latest."** This is what guarantees a producer's wire output is fully determined by its deployed code: if a producer is still running `EmailCreated` v1, it always encodes and stamps v1's ID, regardless of whether v2 has since been registered elsewhere. (If this rule were violated — e.g. a producer fetched "latest" while still running v1-generated code — `avro.Marshal` would either silently fill any new field's declared default, producing messages that look like v2 but never carry real data in that field, or error outright if the new field lacked a default. Pinning to compiled-in content avoids this failure mode entirely.)

## 5. Runtime flow

**Produce** (first message of a process's lifetime for a given schema version):
1. `schemaregistry.Encode` looks up the schema ID matching the process's compiled-in schema (real HTTP call to Apicurio, via `resolveID`).
2. Cached in `idBySubject` for the process's lifetime.
3. Every message after that: `avro.Marshal` + prepend the cached ID + magic byte — pure local computation, no per-message network call.

**Consume**:
1. Read the magic byte + 4-byte schema ID from the message header (`ConfluentHeader.DecodeID`).
2. If this ID hasn't been seen before, fetch the writer schema from the registry (cached in `schemaByID` afterward).
3. Resolve the writer schema against the reader schema (`SchemaCompatibility.Resolve` — see Section 3's caveat on the exact call sequence), then unmarshal — filling in the reader's declared defaults for fields the writer didn't have, and dropping fields the writer had that the reader doesn't.

## 6. Hexagonal boundary fix: `domain/mail` (producer side)

Today, `domain/mail/service.go`'s `buildMailAndEvent` constructs `contracts.EmailCreatedPayload` and calls `json.Marshal` directly — `domain/mail` importing `contracts` for this purpose was accepted as a stopgap in `2026-07-12-outbox-architecture-cleanup-design.md` (see its Section 1 forward-looking note). This spec resolves it: `domain/mail` stops importing any wire-format package entirely.

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

`buildMailAndEvent` builds `OutboxEvent{Key: []byte(strconv.Itoa(raw.OrganizationID)), Fields: EmailReceivedFields{...}}` — no wire-format import, no marshaling.

The mapping from `EmailReceivedFields` into the generated wire struct, and the topic name, move into the infra repository, which imports `eventschemas` (not `libs/schemaregistry` — this is a plain JSON marshal, no registry involved yet; see Section 7):

```go
// infra/postgres/outbox_repository.go
func (c *Client) CreateOutboxEvent(ctx context.Context, mailID int, event mail.OutboxEvent) error {
    wire := eventschemas.EmailCreated{
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
        "topic":   eventschemas.EmailReceivedTopic,
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

This has to be placed carefully. `domain/outbox` (the relay sidecar's own domain package, distinct from `domain/mail`) already defines `Service.ProcessRecord(ctx, rec libsoutbox.Record, maxAttempts int) error`, which today forwards `rec.Payload` straight to `Producer.ProduceSync(ctx, topic string, key, value []byte) error` unchanged — that's correct today because the outbox payload already is the final wire bytes. Putting the JSON-unmarshal-then-Avro-encode step inside `ProcessRecord` would make `domain/outbox` import `eventschemas`/`schemaregistry`, reintroducing the same violation this spec fixes elsewhere. `ProcessRecord` is left untouched; instead, a new infra adapter wraps the `Producer` port so the conversion happens beneath the domain layer, invisibly to it — and this is the adapter where the type/subject/schema coupling described in Section 3 actually lives:

```go
// infra/events/email_created_producer.go — new
type EmailCreatedProducer struct {
    inner  outbox.Producer        // the real franz-go-backed producer (libsoutbox.NewProducer's return value)
    client *schemaregistry.Client // constructed in cmd/main.go from the configured registry client
}

func (p *EmailCreatedProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
    var wire eventschemas.EmailCreated
    if err := json.Unmarshal(value, &wire); err != nil {
        return fmt.Errorf("decode staged outbox payload: %w", err)
    }
    avroBytes, err := schemaregistry.Encode(ctx, p.client, eventschemas.EmailReceivedSubject, eventschemas.EmailCreatedSchema, wire)
    if err != nil {
        return fmt.Errorf("avro encode: %w", err)
    }
    return p.inner.ProduceSync(ctx, topic, key, avroBytes)
}
```

`cmd/main.go` wires `EmailCreatedProducer{inner: libsoutbox.NewProducer(...), client: registryClient}` in as the `outbox.Producer` implementation passed into `outbox.NewService(...)`, in place of the raw producer used today. `domain/outbox.Service.ProcessRecord` requires no code change at all — it keeps calling `s.producer.ProduceSync(ctx, rec.Topic, rec.Key, rec.Payload)` exactly as it does now, unaware that the bytes it passes get converted from JSON to Avro underneath.

This keeps the registry dependency entirely out of the transactional write path, and keeps pending outbox rows human-readable via plain SQL (`SELECT payload FROM outbox WHERE status = 'pending'`) instead of opaque binary — useful for debugging stuck rows.

`libs/outbox`'s `Record`/`Store` types (`Payload []byte`) are unaffected structurally — Postgres returns `JSONB` as bytes over the wire either way; only the column type and what's inside the bytes changes (JSON text instead of pre-Avro-encoded binary).

## 8. Hexagonal boundary fix: `domain/inbox` (consumer side)

The inbox table's `payload BYTEA` column is unchanged — it has to keep storing raw wire bytes (Avro-encoded, Confluent-enveloped) as received from Kafka, since durability must happen before any decode attempt (which requires a registry round-trip) is even possible. Only the *decode call site* moves. (`libs/inbox`'s own poll-and-persist loop, which calls `domain/inbox.Service.Save`, is unaffected — it never touches the payload's contents.)

Verified against the current code (`services/core/internal/domain/inbox/ports.go`, `service.go`, `services/core/internal/infra/inbox/email_received_consumer.go`):

- `Repository` already has `MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error` — this is unchanged.
- `Service` today only exposes `Save`, `FetchBatch`, and `ProcessEmailReceived(ctx context.Context, rec libsinbox.Record, maxAttempts int) error`. `ProcessEmailReceived` currently does the `json.Unmarshal` itself and, on failure, calls `s.repo.MarkFailed(...)` directly — reachable because the domain service already holds `Repository`.
- The infra adapter (`EmailReceivedConsumer.PollOnce`) only holds `Service`, not `Repository`. Once decoding moves to infra, infra needs a way to report a decode failure *before* `ProcessEmailReceived` is ever called — so `Service` gains a `MarkFailed` method with the exact same name and shape as `Repository`'s, simply delegating:

```go
// domain/inbox/ports.go — Service interface, one method added
type Service interface {
    Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error)
    FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error)
    ProcessEmailReceived(ctx context.Context, recordID int64, event EmailReceivedEvent, maxAttempts int) error
    MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error // new
}
```

```go
// domain/inbox/service.go
func (s *service) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
    return s.repo.MarkFailed(ctx, id, procErr, maxAttempts)
}
```

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

`infra/inbox/email_received_consumer.go` gains a `client *schemaregistry.Client` field (constructed in `cmd/main.go`, same as the producer side) and does the decode + type/subject/schema coupling itself, matching the real `PollOnce`'s existing "one bad record does not stop the rest of the batch" behavior — logging and continuing on error rather than aborting:

```go
// infra/inbox/email_received_consumer.go
func (a *EmailReceivedConsumer) PollOnce(ctx context.Context) error {
    records, err := a.service.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
    if err != nil {
        return err
    }

    log := logger.FromContext(ctx)
    for _, rec := range records {
        log.Info().Str("topic", rec.Topic).Int64("inbox_id", rec.ID).Msg("received inbox record")

        var wire eventschemas.EmailCreated
        if err := schemaregistry.Decode(ctx, a.client, rec.Payload, eventschemas.EmailCreatedSchema, &wire); err != nil {
            if markErr := a.service.MarkFailed(ctx, rec.ID, err, a.maxAttempts); markErr != nil {
                log.Error().Err(markErr).Int64("inbox_id", rec.ID).Msg("failed to mark inbox record failed after decode error")
            }
            continue
        }

        event := domaininbox.EmailReceivedEvent{
            Subject: wire.Subject, Body: wire.Body, From: wire.From,
            Date: wire.Date, MessageID: wire.MessageId, OrganizationID: int(wire.OrganizationId),
        }
        if err := a.service.ProcessEmailReceived(ctx, rec.ID, event, a.maxAttempts); err != nil {
            log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to process inbox record")
        }
    }

    return nil
}
```

`domain/inbox/service.go`'s `ProcessEmailReceived` drops its `contracts` import and `json.Unmarshal` call entirely — its signature changes to `ProcessEmailReceived(ctx context.Context, recordID int64, event EmailReceivedEvent, maxAttempts int) error`, and every `rec.ID` reference in its body becomes `recordID`. The rest of the function (categories, LLM extraction/classification, the `UpsertRequest` + `MarkProcessed` transaction, the `MarkFailed`-on-error paths) is otherwise unchanged.

## Testing implications

- `domain/mail/service_test.go`: `buildMailAndEvent`'s test assertions change from checking JSON-marshaled bytes to checking the plain `EmailReceivedFields` struct — no wire-format import needed in this test file anymore.
- `services/email/internal/test/mail/mail_test.go` (integration): asserts the outbox row's `payload` column is valid JSON matching `eventschemas.EmailCreated`'s shape (via `json.Unmarshal` in the test), not raw bytes.
- New integration test for `email-received-producer`: given an outbox row with JSON payload, verify it Avro-encodes and calls `ProduceSync` with a correctly-enveloped payload (magic byte + registered schema ID + Avro bytes) — requires the schema-registry testcontainer (new addition to `internal/test/utils/`).
- `domain/inbox/service_test.go`: `ProcessEmailReceived`'s test cases are rewritten against the new `(ctx, recordID, event, maxAttempts)` signature — no more unmarshal-failure case here, since decode now happens before this method is ever called. New test case for the added `Service.MarkFailed` delegating correctly to `Repository.MarkFailed`.
- New test for `infra/inbox/email_received_consumer.go`: decode failure (malformed/wrong-schema bytes) calls `Service.MarkFailed` and continues to the next record rather than aborting the batch (matching existing `PollOnce` semantics); decode success maps all six fields correctly into `EmailReceivedEvent` and calls `ProcessEmailReceived` with the record's ID.
- `libs/schemaregistry`: new unit tests for `Encode`/`Decode` round-tripping against a real Apicurio testcontainer, including the schema-evolution case (encode with an older schema, decode with a newer one that has an additional defaulted field) — this is what actually proves the `SchemaCompatibility.Resolve` call sequence flagged in Section 3 works as intended, so it should be written and run before that code is considered done, not just before this spec is.
- `libs/eventschemas`: new unit test for `Schemas()` returning a schema that matches the embedded `.avsc` file content.
- Registration CLI (`cmd/registerschemas`): integration test against the Apicurio testcontainer verifying idempotent re-registration (same content twice returns the same ID, no error).

## Explicitly deferred / out of scope

- Publishing `libs/schemaregistry` or `libs/eventschemas` as external Go modules; per-repo CI; separate dev-environment repo for local infra. (Section 3 explains why they're already split as if this were coming, without building it.)
- Multi-schema/multi-topic governance tooling (schema linting in CI, alternate subject-naming strategies).
- Schema-drift alerting/notification tooling.
- Production/staging registry provisioning via OpenTofu, including which registry auth scheme (Basic/Bearer/OIDC) to use there.
- Protobuf/JSON-Schema support in Apicurio (registry supports it; not used here).
- Retrofitting any event type other than `email.received`.
