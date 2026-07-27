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
- **`github.com/twmb/franz-go/pkg/sr`** — used via its `sr.Client` (`LookupSchema`/`SchemaByID`, the content-based lookup-or-register call) **and** its `sr.Serde` convenience type, wrapped rather than reimplemented. `Serde` provides its own wire-envelope framing and per-ID encode/decode dispatch and caching — reimplementing that by hand (an earlier version of this design did) duplicates tested library code for no benefit. The one thing `Serde` doesn't provide is Avro schema evolution: `Serde.Register(id, ...)` binds to one *exact* schema ID, and `Serde.Decode` returns `sr.ErrNotRegistered` for any ID it wasn't told about — it does not fall back to resolving schema differences. `libs/schemaregistry`'s `Serde` wrapper (Section 3) handles exactly that gap, using franz-go's own documented pattern for it: catch `ErrNotRegistered`, fetch the writer schema, resolve it against the reader schema, register the result, and retry — rather than requiring every historical schema ID to be pre-registered by hand.
- Wire format: the Confluent standard envelope (magic byte `0x0` + 4-byte schema ID + Avro payload), via `sr.ConfluentHeader`, since that's what registry-aware tooling expects natively.
- Subject naming: **TopicNameStrategy** — `email.received-value`.

## 3. Two packages: `libs/schemaregistry` (generic) and `libs/eventschemas` (app-specific)

These are kept as two separate packages rather than one, because they change at different rates and have different reuse scope: `libs/schemaregistry` has zero knowledge of what an "email" is and rarely changes; `libs/eventschemas` changes every time an event is added or evolved. Bundling them would mean every schema change also version-bumps "how we talk to the registry," and would mean a future extraction of the generic engine (a legitimate standalone "Kafka + Avro + schema registry" toolkit, useful outside this app) drags planeo's specific event types along with it. Nothing in this spec builds that extraction — both packages just stay self-contained enough that it stays cheap later (only stdlib + `iskorotkov/avro` + `pkg/sr`; no imports from `email`, `core`, or each other beyond `eventschemas`'s own use of the Avro codec types).

### `libs/schemaregistry` — generic engine

```go
// libs/schemaregistry/client.go
// NewClient constructs a schema registry HTTP client. Thin wrapper over
// sr.NewClient purely for a consistent construction point across this
// codebase, matching libs/kafka.NewClient's pattern — adds no behavior.
func NewClient(opts ...sr.ClientOpt) (*sr.Client, error) {
    return sr.NewClient(opts...)
}

// Register performs a content-based lookup-or-register call for subject
// against schema, returning its registry ID. This is the primitive
// cmd/registerschemas (Section 4) uses to walk eventschemas.Schemas()
// generically — it operates on schemas alone, with no Go type involved,
// unlike RegisterEncode/RegisterDecode below.
func Register(ctx context.Context, client *sr.Client, subject string, schema avro.Schema) (int, error) {
    ss, err := client.LookupSchema(ctx, subject, sr.Schema{Schema: schema.String(), Type: sr.TypeAvro})
    if err != nil {
        return 0, err
    }
    return ss.ID, nil
}
```

```go
// libs/schemaregistry/serde.go
// Serde wraps franz-go's own sr.Serde — its wire-envelope framing, per-ID
// dispatch, and caching are used as-is, not reimplemented. What this type
// adds is the one thing sr.Serde doesn't do: resolving Avro schema
// evolution for IDs it wasn't told about in advance.
type Serde struct {
    client *sr.Client
    inner  *sr.Serde
    mu     sync.Mutex
}

func NewSerde(client *sr.Client) *Serde {
    return &Serde{client: client, inner: &sr.Serde{}}
}

// RegisterEncode resolves subject's ID and registers T for encoding.
// Called once per type, at construction time, by whichever infra adapter
// owns that type — this is the coupling site (see below).
func RegisterEncode[T any](ctx context.Context, s *Serde, subject string, schema avro.Schema) error {
    id, err := Register(ctx, s.client, subject, schema)
    if err != nil {
        return err
    }
    s.inner.Register(id, *new(T),
        sr.EncodeFn(func(v any) ([]byte, error) { return avro.Marshal(schema, v) }),
        sr.DecodeFn(func(b []byte, v any) error { return avro.Unmarshal(schema, b, v) }),
    )
    return nil
}

// RegisterDecode resolves subject's ID for schema and registers T for
// decoding under that exact ID. Since this ID corresponds to our own
// compiled-in schema, no writer/reader resolution is needed for it — a
// message tagged with this ID was necessarily written with this exact
// schema. This doubles as a startup fail-fast check: construction fails
// immediately if the registry is unreachable or has no entry for our own
// schema, rather than only surfacing that on the first real message.
func RegisterDecode[T any](ctx context.Context, s *Serde, subject string, schema avro.Schema) error {
    id, err := Register(ctx, s.client, subject, schema)
    if err != nil {
        return err
    }
    s.mu.Lock()
    s.inner.Register(id, *new(T), sr.DecodeFn(func(b []byte, v any) error {
        return avro.Unmarshal(schema, b, v)
    }))
    s.mu.Unlock()
    return nil
}

func Encode[T any](s *Serde, v T) ([]byte, error) {
    return s.inner.Encode(v)
}

// Decode delegates to sr.Serde first — the fast path for any ID already
// registered via RegisterEncode/RegisterDecode or a prior Decode call on
// this same Serde. On ErrNotRegistered — an ID this process hasn't seen —
// it fetches that writer schema, resolves it against readerSchema,
// registers the result, and retries. This is franz-go's own documented
// pattern for this situation ("pre-register schema ids and values you will
// consume, or ... discover the schema every time you receive an
// ErrNotRegistered error from decode"), not a workaround.
func Decode[T any](ctx context.Context, s *Serde, b []byte, readerSchema avro.Schema, v *T) error {
    err := s.inner.Decode(b, v)
    if !errors.Is(err, sr.ErrNotRegistered) {
        return err
    }

    id, _, hdrErr := (&sr.ConfluentHeader{}).DecodeID(b)
    if hdrErr != nil {
        return hdrErr
    }
    raw, fetchErr := s.client.SchemaByID(ctx, id)
    if fetchErr != nil {
        return fetchErr
    }
    writerSchema, parseErr := avro.Parse(raw.Schema)
    if parseErr != nil {
        return parseErr
    }
    resolved, resolveErr := avro.NewSchemaCompatibility().Resolve(readerSchema, writerSchema)
    if resolveErr != nil {
        return resolveErr
    }

    s.mu.Lock()
    s.inner.Register(id, *new(T), sr.DecodeFn(func(bb []byte, vv any) error { return avro.Unmarshal(resolved, bb, vv) }))
    s.mu.Unlock()

    return s.inner.Decode(b, v)
}
```

(The exact `Resolve`→`Unmarshal` call sequence inside the `ErrNotRegistered` branch was verified conceptually — two schemas are genuinely required, `Resolve` is the documented API for it — but not compiled against the real library; this is the piece to double-check first if implementation doesn't match.)

Each service adds a `SCHEMA_REGISTRY_URL` env var to its `internal/config/config.go` (same pattern as `NATS_URL`, `DB_*`, `KC_*`), and `cmd/main.go` constructs the client once — `schemaregistry.NewClient(sr.URLs(cfg.SchemaRegistryURL))` for local dev, with `sr.BasicAuth(...)`/`sr.BearerToken(...)`/`sr.DialTLSConfig(...)` added once a non-local registry's auth scheme is decided (out of scope; see Non-goals). This mirrors how every other external client in this codebase is wired, and means moving from the local registry to a remote one is a config change, not a code change.

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

Neither package couples a specific type to the generic engine — that happens explicitly in the infra adapters that need it (Sections 7 and 8), which construct their own `*schemaregistry.Serde` and call `schemaregistry.RegisterEncode[eventschemas.EmailCreated](ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailCreatedSchema)` (producer) or `RegisterDecode` (consumer) at construction time. This is deliberate: an adapter's job in hexagonal architecture is exactly to wire concrete things together, so this is where "which type, which subject, which schema" belongs — not hidden behind a per-type wrapper function, and not pushed into either generic package.

## 4. Schema registration

Schemas are registered via a **content-based lookup-or-register** call — the registry is asked "what ID matches this exact schema content," which returns the existing ID if it's already registered rather than creating a duplicate. This is never done via producer auto-registration at runtime; auto-registration would let a running process silently publish an unreviewed schema change.

New Go command, `libs/eventschemas/cmd/registerschemas/main.go`, using `schemaregistry.Register` (Section 3 — the non-generic, schema-only primitive, since this CLI walks `eventschemas.Schemas()`'s `map[string]avro.Schema` with no concrete Go type available per entry), registering each subject against the configured registry URL. Wired as a new Task target, `task register:schemas`, called from `task up` immediately after the existing migration step — same "safe to rerun every time" property as goose migrations: registering an already-identical schema is a no-op that returns the existing ID.

**Producers always resolve their schema ID by matching their own compiled-in schema content, never by fetching "latest."** This is what guarantees a producer's wire output is fully determined by its deployed code: if a producer is still running `EmailCreated` v1, it always encodes and stamps v1's ID, regardless of whether v2 has since been registered elsewhere. (If this rule were violated — e.g. a producer fetched "latest" while still running v1-generated code — `avro.Marshal` would either silently fill any new field's declared default, producing messages that look like v2 but never carry real data in that field, or error outright if the new field lacked a default. Pinning to compiled-in content avoids this failure mode entirely.)

## 5. Runtime flow

**Produce** (at adapter construction, then steady state):
1. At startup, `schemaregistry.RegisterEncode[EmailCreated]` resolves the schema ID matching the process's compiled-in schema (real HTTP call to Apicurio) and registers it with the underlying `sr.Serde`. This is eager and can fail construction — see Section 7.
2. Every `Encode` call after that is `sr.Serde.Encode`'s own cached path: `avro.Marshal` + the registered ID + magic byte, pure local computation, no per-message network call.

**Consume** (at adapter construction, then steady state, then on schema change):
1. At startup, `schemaregistry.RegisterDecode[EmailCreated]` resolves the ID matching the process's own compiled-in schema and registers it for direct decoding (no resolution needed, since a message with this exact ID was necessarily written with this exact schema) — this also fails construction fast if the registry is unreachable, rather than only surfacing that on the first message (see Section 8).
2. Any message whose ID matches that registration decodes via `sr.Serde`'s own cached fast path.
3. Any message with a *different* ID (an older or newer schema version this Serde hasn't registered) causes `sr.Serde.Decode` to return `ErrNotRegistered`; `schemaregistry.Decode` catches this, fetches that writer schema, resolves it against the reader schema (`SchemaCompatibility.Resolve` — see Section 3's caveat on the exact call sequence), registers the resolved decode function under that ID, and retries — filling in the reader's declared defaults for fields the writer didn't have, and dropping fields the writer had that the reader doesn't. Every subsequent message with that same ID then also hits the fast path.

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
    inner outbox.Producer        // the real franz-go-backed producer (libsoutbox.NewProducer's return value)
    serde *schemaregistry.Serde
}

// NewEmailCreatedProducer registers EmailCreated for encoding once, at
// construction time — this is the coupling site described in Section 3.
// It takes ctx and can fail, since registration is a real registry
// round-trip, not something deferred to the first ProduceSync call.
func NewEmailCreatedProducer(ctx context.Context, inner outbox.Producer, client *sr.Client) (*EmailCreatedProducer, error) {
    serde := schemaregistry.NewSerde(client)
    if err := schemaregistry.RegisterEncode[eventschemas.EmailCreated](
        ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailCreatedSchema,
    ); err != nil {
        return nil, fmt.Errorf("registering EmailCreated for encoding: %w", err)
    }
    return &EmailCreatedProducer{inner: inner, serde: serde}, nil
}

func (p *EmailCreatedProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
    var wire eventschemas.EmailCreated
    if err := json.Unmarshal(value, &wire); err != nil {
        return fmt.Errorf("decode staged outbox payload: %w", err)
    }
    avroBytes, err := schemaregistry.Encode(p.serde, wire)
    if err != nil {
        return fmt.Errorf("avro encode: %w", err)
    }
    return p.inner.ProduceSync(ctx, topic, key, avroBytes)
}
```

`cmd/main.go` calls `events.NewEmailCreatedProducer(ctx, libsoutbox.NewProducer(...), registryClient)` and fails startup (`log.Fatal`) if it errors, then passes the result into `outbox.NewService(...)` as the `outbox.Producer` implementation, in place of the raw producer used today. `domain/outbox.Service.ProcessRecord` requires no code change at all — it keeps calling `s.producer.ProduceSync(ctx, rec.Topic, rec.Key, rec.Payload)` exactly as it does now, unaware that the bytes it passes get converted from JSON to Avro underneath.

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

`infra/inbox/email_received_consumer.go` gains a `serde *schemaregistry.Serde` field. Its constructor registers `EmailCreated` for decoding up front via `RegisterDecode` — this is both the coupling site (matching the producer side) and a startup fail-fast check, per Section 5:

```go
// infra/inbox/email_received_consumer.go
type EmailReceivedConsumer struct {
    service     domaininbox.Service
    serde       *schemaregistry.Serde
    topic       string
    instanceID  string
    batchSize   int
    claimTTL    time.Duration
    maxAttempts int
}

func NewEmailReceivedConsumer(
    ctx context.Context,
    service domaininbox.Service,
    client *sr.Client,
    topic string,
    instanceID string,
    batchSize int,
    maxAttempts int,
    claimTTL time.Duration,
) (*EmailReceivedConsumer, error) {
    serde := schemaregistry.NewSerde(client)
    if err := schemaregistry.RegisterDecode[eventschemas.EmailCreated](
        ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailCreatedSchema,
    ); err != nil {
        return nil, fmt.Errorf("registering EmailCreated for decoding: %w", err)
    }
    return &EmailReceivedConsumer{
        service: service, serde: serde, topic: topic, instanceID: instanceID,
        batchSize: batchSize, claimTTL: claimTTL, maxAttempts: maxAttempts,
    }, nil
}

// PollOnce claims a batch of pending inbox rows and processes each in turn.
// One bad record does not stop the rest of the batch.
func (a *EmailReceivedConsumer) PollOnce(ctx context.Context) error {
    records, err := a.service.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
    if err != nil {
        return err
    }

    log := logger.FromContext(ctx)
    for _, rec := range records {
        log.Info().Str("topic", rec.Topic).Int64("inbox_id", rec.ID).Msg("received inbox record")

        var wire eventschemas.EmailCreated
        if err := schemaregistry.Decode(ctx, a.serde, rec.Payload, eventschemas.EmailCreatedSchema, &wire); err != nil {
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
- `libs/schemaregistry`: new tests against a real Apicurio testcontainer — a basic `RegisterEncode`+`Encode`/`Decode` round trip; and, critically, a schema-evolution case using a *fresh* `Serde` (simulating a consumer process that has never seen the producer's ID) to decode a message written with an older schema into a newer reader schema with an additional defaulted field. That second test is what actually exercises the `ErrNotRegistered` → fetch → `SchemaCompatibility.Resolve` → retry path from Section 3, so it should be written and passing before that code is considered done, not just before this spec is.
- `libs/eventschemas`: new unit test for `Schemas()` returning a schema that matches the embedded `.avsc` file content.
- Registration CLI (`cmd/registerschemas`): integration test against the Apicurio testcontainer verifying idempotent re-registration (same content twice returns the same ID, no error).

## Explicitly deferred / out of scope

- Publishing `libs/schemaregistry` or `libs/eventschemas` as external Go modules; per-repo CI; separate dev-environment repo for local infra. (Section 3 explains why they're already split as if this were coming, without building it.)
- Multi-schema/multi-topic governance tooling (schema linting in CI, alternate subject-naming strategies).
- Schema-drift alerting/notification tooling.
- Production/staging registry provisioning via OpenTofu, including which registry auth scheme (Basic/Bearer/OIDC) to use there.
- Protobuf/JSON-Schema support in Apicurio (registry supports it; not used here).
- Retrofitting any event type other than `email.received`.
