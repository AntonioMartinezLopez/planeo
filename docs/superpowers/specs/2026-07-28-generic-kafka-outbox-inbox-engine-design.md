# Generic Kafka Outbox/Inbox Engine (v1)

## Purpose

`2026-07-25-kafka-schema-registry-avro-design.md` and its follow-up consolidation refactor produced two email-received-specific types — `services/email/internal/infra/outbox.EmailReceivedProducer` and `services/core/internal/infra/inbox.EmailReceivedConsumer` — that each own a full fetch/decode-or-encode/send-or-process/mark loop, wired together by hand in each service's `cmd/main.go` (raw Kafka client construction, schema-registry client construction, `RegisterEncode`/`RegisterDecode`, `Encoder`/`Decoder` construction, then the adapter itself).

Both loops are now proven twice over (once per direction) with identical shape, and neither has any real "email" knowledge left in its control flow — only in the type parameter and the three or four function values it's handed. This spec pushes that entire engine — Kafka client construction, schema-registry client construction and registration, the ticker loop, encode/decode — down into `libs/outbox`/`libs/inbox` as generic types parameterized over the wire type `T`. Each service keeps only a thin, named adapter that fixes the "which topic/subject/schema" identity and wires its domain service's methods in as plain function values.

## Goals

- A single generic `libs/outbox.Producer[T]` (and `libs/inbox.Consumer[T]`) that any future topic/service can construct from: topic, subject, schema, registry connection config, Kafka connection config, poll/batch/attempt/TTL config, and a small set of function-shaped dependencies bound to the domain service.
- Collapse each sidecar's `cmd/main.go` to config-building + one constructor call + `Run` (+ `Close` for the producer) — no direct `kafka.NewClient`/`schemaregistry.NewClient`/`RegisterEncode`/`RegisterDecode`/`NewEncoder`/`NewDecoder` calls left there.
- Keep a thin, named, discoverable adapter per service (`infra/outbox.NewEmailReceivedProducer`, `infra/inbox.NewEmailReceivedConsumer`) so "the email-received producer/consumer" still has an obvious home in each service's own tree, even though it now only maps identity and wires function values.
- Preserve the existing, deliberate per-topic method naming on domain services (`ProcessEmailReceived`, not a generic `Process`) — resolved via function-value injection (Go method values match by signature, not name) rather than requiring domain services to implement a uniformly-named interface.
- Keep the fast, fake-based unit tests introduced in the consolidation refactor working — now living in `libs/outbox`/`libs/inbox`, generic over a test type instead of hardcoded to `eventschemas.EmailReceived`.
- `libs/outbox.BuildConfig`/`libs/inbox.BuildConfig` (Section 1, 2), and the thin per-service adapters' own `Config` one level up (Section 3), use only flat scalar fields — no `kafka.Config`, no `schemaregistry.Config` — anywhere on the path from `cmd/main.go` down to `NewFromConfig`. `libs/outbox`/`libs/inbox` still import both internally (unavoidable — they're the ones actually calling `kafka.NewClient`/`schemaregistry.NewClient`), but that's the only place either type gets constructed; `cmd/main.go` and the thin adapters need neither import.
- Zero change to `domain/outbox`/`domain/inbox` themselves. `domain/outbox`'s three methods bind directly as `Deps[T]` function values with no adapter needed. `domain/inbox`'s `Save`/`FetchBatch`/`MarkFailed` do too — but `ProcessEmailReceived` takes `domaininbox.EmailReceivedEvent` (the domain-owned event type), not `eventschemas.EmailReceived` (the wire type `Consumer[T]` decodes into), by the original Kafka spec's own deliberate design (Section 8 there: keep `domain/inbox` free of wire-format types). `Consumer[T]`'s single type parameter is the wire type throughout, so this mapping can't collapse away — it moves into the thin adapter as a small closure (Section 3), the same six-field copy that lived in `infra/inbox.EmailReceivedConsumer.processRecord` before this spec, just relocated.

## Non-goals

- A second real topic/event type. This generalizes the shape; it doesn't add a new topic.
- Multiple topics sharing one `Producer[T]`/`Consumer[T]` instance — one instance still handles exactly one topic, one type.
- Any change to wire format, schema content, DB schema, or `ProcessEmailReceived`'s internal business logic.
- Registry/Kafka auth beyond what already exists — still deferred per the original Kafka spec's Non-goals.
- Renaming `domain/inbox.Service.ProcessEmailReceived` to a generic name — explicitly rejected in favor of function-value injection (see Goals).

## 1. `libs/outbox`: renames and the new `Producer[T]`

The existing exported names `Producer` (the raw-sender interface) and `NewProducer` (its constructor) collide with the new top-level generic type this spec introduces. Both are renamed; `kafkaProducer` (the unexported `*kgo.Client`-backed implementation) is untouched:

| Before | After |
|---|---|
| `type Producer interface { ProduceSync(...) error }` | `type RawProducer interface { ProduceSync(...) error }` |
| `func NewProducer(client syncProducer) Producer` | `func NewKafkaProducer(client syncProducer) RawProducer` |

```go
// libs/outbox/producer.go — renamed, otherwise unchanged
type RawProducer interface {
    ProduceSync(ctx context.Context, topic string, key, value []byte) error
}

func NewKafkaProducer(client syncProducer) RawProducer { /* unchanged body */ }
```

New file, `libs/outbox/generic_producer.go`:

```go
package outbox

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "planeo/libs/kafka"
    "planeo/libs/logger"
    "planeo/libs/schemaregistry"

    avro "github.com/iskorotkov/avro/v2"
)

// Encoder Avro-encodes a decoded T value. Declared here (rather than
// depending on *schemaregistry.Encoder[T] directly) so it can be mocked in
// unit tests without a live registry; *schemaregistry.Encoder[T] satisfies
// this structurally, no wrapper needed.
type Encoder[T any] interface {
    Encode(v T) ([]byte, error)
}

// Deps are Producer[T]'s business-side collaborators — the domain service's
// three methods, as function values. Method values match by signature, not
// name: a domain service can call this method anything (ProcessRecord,
// whatever a future topic needs) and it still plugs in here unchanged.
type Deps[T any] struct {
    FetchBatch    func(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]Record, error)
    MarkProcessed func(ctx context.Context, id int64) error
    MarkFailed    func(ctx context.Context, id int64, err error, maxAttempts int) error
}

// Config is Producer[T]'s scalar, topic/instance-specific configuration —
// independent of how the underlying connections were built.
type Config struct {
    Topic, InstanceID string
    BatchSize, MaxAttempts int
    ClaimTTL, PollInterval time.Duration
}

// Producer[T] owns a full outbox produce loop for one topic and one wire
// type T: poll the domain service for a batch of pending rows, JSON-decode
// each row's staged payload into T, Avro-encode it, send it, and record the
// outcome. Zero knowledge of what T actually is beyond its JSON/Avro tags.
type Producer[T any] struct {
    rawProducer RawProducer
    encoder     Encoder[T]
    service     Deps[T]
    cfg         Config
    closeFn     func() error // no-op unless built via NewFromConfig
}

// New is the low-level constructor: takes an already-built raw producer and
// encoder. No I/O, no error return — this is what unit tests use, with
// fakes, no live Kafka or registry required.
func New[T any](rawProducer RawProducer, encoder Encoder[T], deps Deps[T], cfg Config) *Producer[T] {
    return &Producer[T]{
        rawProducer: rawProducer, encoder: encoder,
        service: deps, cfg: cfg, closeFn: func() error { return nil },
    }
}

// BuildConfig is the connection-level configuration NewFromConfig needs to
// build the raw Kafka producer and schema-registry encoder itself. Flat
// scalar fields only — no kafka.Config or schemaregistry.Config — so a
// caller populating this doesn't need to import either package just to
// wrap one or five values in a struct. libs/outbox still imports both
// internally (it has to, to actually call kafka.NewClient/schemaregistry.NewClient);
// this only decouples the caller's Config shape from theirs.
type BuildConfig struct {
    KafkaBrokers    []string
    KafkaUsername   string
    KafkaPassword   string
    KafkaMechanism  string
    KafkaTLSEnabled bool
    RegistryURL     string
    Subject         string
    Schema          avro.Schema
}

// NewFromConfig is the high-level, one-call constructor most callers use:
// builds the Kafka client, the schema-registry client, registers T for
// encoding, and returns a ready-to-run Producer[T]. The caller owns calling
// Close() (it owns the Kafka client's lifecycle now, unlike New).
func NewFromConfig[T any](ctx context.Context, build BuildConfig, deps Deps[T], cfg Config) (*Producer[T], error) {
    kafkaClient, err := kafka.NewClient(kafka.Config{
        Brokers: build.KafkaBrokers, Username: build.KafkaUsername,
        Password: build.KafkaPassword, Mechanism: build.KafkaMechanism, TLSEnabled: build.KafkaTLSEnabled,
    })
    if err != nil {
        return nil, fmt.Errorf("connect to kafka: %w", err)
    }

    registryClient, err := schemaregistry.NewClient(schemaregistry.Config{URL: build.RegistryURL})
    if err != nil {
        kafkaClient.Close()
        return nil, fmt.Errorf("create schema registry client: %w", err)
    }
    serde := schemaregistry.NewSerde(registryClient)
    if err := schemaregistry.RegisterEncode[T](ctx, serde, build.Subject, build.Schema); err != nil {
        kafkaClient.Close()
        return nil, fmt.Errorf("register %s for encoding: %w", build.Subject, err)
    }

    p := New[T](NewKafkaProducer(kafkaClient), schemaregistry.NewEncoder[T](serde), deps, cfg)
    // (*kgo.Client).Close returns nothing, not an error — wrap it to match
    // closeFn's signature.
    p.closeFn = func() error { kafkaClient.Close(); return nil }
    return p, nil
}

// Close releases the Kafka connection. A Producer[T] built via New (tests,
// or a caller supplying its own already-managed client) has nothing to
// release — Close is a no-op there.
func (p *Producer[T]) Close() error { return p.closeFn() }

// Run polls on a fixed interval until ctx is cancelled. Replaces the
// deleted libs/outbox.Runner — this type now owns its own loop directly.
func (p *Producer[T]) Run(ctx context.Context) error {
    log := logger.FromContext(ctx)
    ticker := time.NewTicker(p.cfg.PollInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := p.PollOnce(ctx); err != nil {
                log.Error().Err(err).Msg("outbox producer poll failed")
            }
        }
    }
}

// PollOnce claims a batch of pending rows and processes each in turn. One
// bad record does not stop the rest of the batch.
func (p *Producer[T]) PollOnce(ctx context.Context) error {
    records, err := p.service.FetchBatch(ctx, p.cfg.Topic, p.cfg.InstanceID, p.cfg.BatchSize, p.cfg.ClaimTTL)
    if err != nil {
        return err
    }
    log := logger.FromContext(ctx)
    for _, rec := range records {
        log.Info().Str("topic", rec.Topic).Int64("outbox_id", rec.ID).Msg("sending outbox record")
        p.processRecord(ctx, rec)
    }
    return nil
}

func (p *Producer[T]) processRecord(ctx context.Context, rec Record) {
    log := logger.FromContext(ctx)
    if err := p.produce(ctx, rec); err != nil {
        if markErr := p.service.MarkFailed(ctx, rec.ID, err, p.cfg.MaxAttempts); markErr != nil {
            log.Error().Err(markErr).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record failed after send error")
        }
        return
    }
    if err := p.service.MarkProcessed(ctx, rec.ID); err != nil {
        log.Error().Err(err).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record processed")
    }
}

func (p *Producer[T]) produce(ctx context.Context, rec Record) error {
    var v T
    if err := json.Unmarshal(rec.Payload, &v); err != nil {
        return fmt.Errorf("decode staged outbox payload: %w", err)
    }
    avroBytes, err := p.encoder.Encode(v)
    if err != nil {
        return fmt.Errorf("avro encode: %w", err)
    }
    return p.rawProducer.ProduceSync(ctx, rec.Topic, rec.Key, avroBytes)
}
```

## 2. `libs/inbox`: renames and the new `Consumer[T]`

Same collision, same fix. The existing raw Kafka-to-DB ingestion type is `Consumer`; it's renamed so the new top-level generic type can take that name:

| Before | After |
|---|---|
| `type Consumer struct { ... }` | `type KafkaConsumer struct { ... }` |
| `func NewConsumer(client fetchClient, topic string, store saver) *Consumer` | `func NewKafkaConsumer(client fetchClient, topic string, store saver) *KafkaConsumer` |
| (implicit — always called directly) | `type RawConsumer interface { Run(ctx context.Context) error }` — new, satisfied structurally by `*KafkaConsumer` |

`consumer.go`'s internals (`fetchClient`, `saver`, the fetch/commit loop) are otherwise unchanged.

New file, `libs/inbox/generic_consumer.go`:

```go
package inbox

import (
    "context"
    "fmt"
    "time"

    "planeo/libs/kafka"
    "planeo/libs/logger"
    "planeo/libs/schemaregistry"

    avro "github.com/iskorotkov/avro/v2"
    "github.com/twmb/franz-go/pkg/kgo"
)

// Decoder Avro-decodes a raw payload into a T value. Declared here (rather
// than depending on *schemaregistry.Decoder[T] directly) so it can be
// mocked in unit tests without a live registry; *schemaregistry.Decoder[T]
// satisfies this structurally, no wrapper needed.
type Decoder[T any] interface {
    Decode(ctx context.Context, b []byte) (T, error)
}

// Deps are Consumer[T]'s business-side collaborators, as function values —
// same method-value binding as libs/outbox.Deps[T]. Process is deliberately
// not required to be named "Process": a domain service's ProcessEmailReceived
// (or any future topic's own name) plugs in as-is.
type Deps[T any] struct {
    FetchBatch func(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]Record, error)
    Process    func(ctx context.Context, id int64, event T, maxAttempts int) error
    MarkFailed func(ctx context.Context, id int64, err error, maxAttempts int) error
}

// Config is Consumer[T]'s scalar, topic/instance-specific configuration.
type Config struct {
    Topic, InstanceID string
    BatchSize, MaxAttempts int
    ClaimTTL, PollInterval time.Duration
}

// Consumer[T] owns a full inbox consume loop for one topic and one wire
// type T: the raw Kafka-to-DB ingestion loop, plus polling the domain
// service for a batch of claimed rows, Avro-decoding each payload, and
// handing the decoded event to the domain service.
type Consumer[T any] struct {
    rawConsumer RawConsumer
    decoder     Decoder[T]
    service     Deps[T]
    cfg         Config
}

// New is the low-level constructor: takes an already-built raw Kafka
// consumer and decoder. No I/O, no error return.
func New[T any](rawConsumer RawConsumer, decoder Decoder[T], deps Deps[T], cfg Config) *Consumer[T] {
    return &Consumer[T]{rawConsumer: rawConsumer, decoder: decoder, service: deps, cfg: cfg}
}

// SaveFunc is the raw Kafka-to-DB persistence step (dedup by Kafka
// coordinate) the ingestion loop needs. It's a parameter of NewFromConfig
// only, not part of Deps[T] — New's caller supplies an already-built
// RawConsumer that has this baked in from its own construction, so Deps[T]
// would otherwise carry a field New never reads.
//
// NewKafkaConsumer's third parameter is the unexported saver interface
// (Save(ctx, topic, partition, offset, payload) (bool, error)) — a bare
// func value doesn't satisfy that on its own, so SaveFunc carries a Save
// method that calls itself, the same http.HandlerFunc-style adapter used
// throughout the standard library.
type SaveFunc func(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error)

func (f SaveFunc) Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error) {
    return f(ctx, topic, partition, offset, payload)
}

// BuildConfig is the connection-level configuration NewFromConfig needs to
// build the raw Kafka consumer and schema-registry decoder itself. Flat
// scalar fields only, for the same reason as libs/outbox.BuildConfig — see
// its doc comment.
type BuildConfig struct {
    KafkaBrokers    []string
    KafkaUsername   string
    KafkaPassword   string
    KafkaMechanism  string
    KafkaTLSEnabled bool
    GroupName       string
    RegistryURL     string
    Subject         string
    Schema          avro.Schema
}

// NewFromConfig is the high-level, one-call constructor most callers use.
func NewFromConfig[T any](ctx context.Context, build BuildConfig, save SaveFunc, deps Deps[T], cfg Config) (*Consumer[T], error) {
    kafkaClient, err := kafka.NewClient(kafka.Config{
        Brokers: build.KafkaBrokers, Username: build.KafkaUsername,
        Password: build.KafkaPassword, Mechanism: build.KafkaMechanism, TLSEnabled: build.KafkaTLSEnabled,
    },
        kgo.ConsumerGroup(build.GroupName),
        kgo.ConsumeTopics(cfg.Topic),
        kgo.DisableAutoCommit(),
    )
    if err != nil {
        return nil, fmt.Errorf("connect to kafka: %w", err)
    }

    registryClient, err := schemaregistry.NewClient(schemaregistry.Config{URL: build.RegistryURL})
    if err != nil {
        return nil, fmt.Errorf("create schema registry client: %w", err)
    }
    serde := schemaregistry.NewSerde(registryClient)
    if err := schemaregistry.RegisterDecode[T](ctx, serde, build.Subject, build.Schema); err != nil {
        return nil, fmt.Errorf("register %s for decoding: %w", build.Subject, err)
    }

    rawConsumer := NewKafkaConsumer(kafkaClient, cfg.Topic, save)
    decoder := schemaregistry.NewDecoder[T](serde, build.Schema)
    return New[T](rawConsumer, decoder, deps, cfg), nil
}

// Run starts the raw Kafka ingestion loop (non-blocking — it starts its own
// goroutine, which closes the Kafka client itself on ctx.Done(); no
// separate Close is needed on Consumer[T]) and then blocks running its own
// poll ticker over the inbox table until ctx is cancelled.
func (c *Consumer[T]) Run(ctx context.Context) error {
    if err := c.rawConsumer.Run(ctx); err != nil {
        return err
    }
    log := logger.FromContext(ctx)
    ticker := time.NewTicker(c.cfg.PollInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := c.PollOnce(ctx); err != nil {
                log.Error().Err(err).Msg("inbox consumer poll failed")
            }
        }
    }
}

// PollOnce claims a batch of pending inbox rows and processes each in turn.
// One bad record does not stop the rest of the batch.
func (c *Consumer[T]) PollOnce(ctx context.Context) error {
    records, err := c.service.FetchBatch(ctx, c.cfg.Topic, c.cfg.InstanceID, c.cfg.BatchSize, c.cfg.ClaimTTL)
    if err != nil {
        return err
    }
    log := logger.FromContext(ctx)
    for _, rec := range records {
        log.Info().Str("topic", rec.Topic).Int64("inbox_id", rec.ID).Msg("received inbox record")
        c.processRecord(ctx, rec.ID, rec.Payload)
    }
    return nil
}

func (c *Consumer[T]) processRecord(ctx context.Context, recordID int64, payload []byte) {
    log := logger.FromContext(ctx)
    event, err := c.decoder.Decode(ctx, payload)
    if err != nil {
        if markErr := c.service.MarkFailed(ctx, recordID, err, c.cfg.MaxAttempts); markErr != nil {
            log.Error().Err(markErr).Int64("inbox_id", recordID).Msg("failed to mark inbox record failed after decode error")
        }
        return
    }
    if err := c.service.Process(ctx, recordID, event, c.cfg.MaxAttempts); err != nil {
        log.Error().Err(err).Int64("inbox_id", recordID).Msg("failed to process inbox record")
    }
}
```

## 3. Thin per-service adapters

Each service's `infra/outbox`/`infra/inbox` package shrinks to a type alias, a `Config` scoped to what varies per deployment (no `Topic`/`Subject`/`Schema` — this package's whole job is fixing those), and a constructor that wires the domain service's methods in as function values:

```go
// services/email/internal/infra/outbox/email_received_producer.go
package outbox

import (
    "context"
    "time"

    "planeo/libs/eventschemas"
    libsoutbox "planeo/libs/outbox"
    domainoutbox "planeo/services/email/internal/domain/outbox"
)

type EmailReceivedProducer = libsoutbox.Producer[eventschemas.EmailReceived]

// Config is flat all the way through — no kafka.Config, no schemaregistry.Config
// — so this package, and cmd/main.go one level up, need neither import just
// to build one.
type Config struct {
    KafkaBrokers    []string
    KafkaUsername   string
    KafkaPassword   string
    KafkaMechanism  string
    KafkaTLSEnabled bool
    RegistryURL     string
    InstanceID      string
    BatchSize, MaxAttempts int
    ClaimTTL, PollInterval time.Duration
}

func NewEmailReceivedProducer(ctx context.Context, service domainoutbox.Service, cfg Config) (*EmailReceivedProducer, error) {
    return libsoutbox.NewFromConfig[eventschemas.EmailReceived](ctx,
        libsoutbox.BuildConfig{
            KafkaBrokers:    cfg.KafkaBrokers,
            KafkaUsername:   cfg.KafkaUsername,
            KafkaPassword:   cfg.KafkaPassword,
            KafkaMechanism:  cfg.KafkaMechanism,
            KafkaTLSEnabled: cfg.KafkaTLSEnabled,
            RegistryURL:     cfg.RegistryURL,
            Subject:         eventschemas.EmailReceivedSubject,
            Schema:          eventschemas.EmailReceivedSchema,
        },
        libsoutbox.Deps[eventschemas.EmailReceived]{
            FetchBatch:    service.FetchBatch,
            MarkProcessed: service.MarkProcessed,
            MarkFailed:    service.MarkFailed,
        },
        libsoutbox.Config{
            Topic: eventschemas.EmailReceivedTopic, InstanceID: cfg.InstanceID,
            BatchSize: cfg.BatchSize, MaxAttempts: cfg.MaxAttempts,
            ClaimTTL: cfg.ClaimTTL, PollInterval: cfg.PollInterval,
        },
    )
}
```

```go
// services/core/internal/infra/inbox/email_received_consumer.go
package inbox

import (
    "context"
    "time"

    "planeo/libs/eventschemas"
    libsinbox "planeo/libs/inbox"
    domaininbox "planeo/services/core/internal/domain/inbox"
)

type EmailReceivedConsumer = libsinbox.Consumer[eventschemas.EmailReceived]

type Config struct {
    KafkaBrokers    []string
    KafkaUsername   string
    KafkaPassword   string
    KafkaMechanism  string
    KafkaTLSEnabled bool
    GroupName       string
    RegistryURL     string
    InstanceID      string
    BatchSize, MaxAttempts int
    ClaimTTL, PollInterval time.Duration
}

func NewEmailReceivedConsumer(ctx context.Context, service domaininbox.Service, cfg Config) (*EmailReceivedConsumer, error) {
    return libsinbox.NewFromConfig[eventschemas.EmailReceived](ctx,
        libsinbox.BuildConfig{
            KafkaBrokers:    cfg.KafkaBrokers,
            KafkaUsername:   cfg.KafkaUsername,
            KafkaPassword:   cfg.KafkaPassword,
            KafkaMechanism:  cfg.KafkaMechanism,
            KafkaTLSEnabled: cfg.KafkaTLSEnabled,
            GroupName:       cfg.GroupName,
            RegistryURL:     cfg.RegistryURL,
            Subject:         eventschemas.EmailReceivedSubject,
            Schema:          eventschemas.EmailReceivedSchema,
        },
        service.Save,
        libsinbox.Deps[eventschemas.EmailReceived]{
            FetchBatch: service.FetchBatch,
            // ProcessEmailReceived takes domaininbox.EmailReceivedEvent, the
            // domain-owned event type — not eventschemas.EmailReceived, the
            // wire type Consumer[T] decodes into. This six-field mapping is
            // the one piece of logic this adapter carries; it existed
            // before this spec too (infra/inbox.EmailReceivedConsumer.processRecord),
            // just inline in the loop instead of at the boundary.
            Process: func(ctx context.Context, id int64, wire eventschemas.EmailReceived, maxAttempts int) error {
                return service.ProcessEmailReceived(ctx, id, domaininbox.EmailReceivedEvent{
                    Subject:        wire.Subject,
                    Body:           wire.Body,
                    From:           wire.From,
                    Date:           wire.Date,
                    MessageID:      wire.MessageID,
                    OrganizationID: wire.OrganizationID,
                }, maxAttempts)
            },
            MarkFailed: service.MarkFailed,
        },
        libsinbox.Config{
            Topic: eventschemas.EmailReceivedTopic, InstanceID: cfg.InstanceID,
            BatchSize: cfg.BatchSize, MaxAttempts: cfg.MaxAttempts,
            ClaimTTL: cfg.ClaimTTL, PollInterval: cfg.PollInterval,
        },
    )
}
```

Note `service.ProcessEmailReceived` passed directly as the `Process` function value — no rename, no shim, per Goals.

Neither thin adapter imports `libs/kafka` or `libs/schemaregistry` at all now. `kafka.Config`/`schemaregistry.Config` are fully internal to `libs/outbox`/`libs/inbox` — constructed nowhere else, in exactly one place each (`NewFromConfig`).

## 4. `cmd/main.go` changes

**Producer** (`services/email/cmd/email-received-producer/main.go`):

```go
db := postgres.NewClient(ctx, cfg.DatabaseConfig())
defer db.Close()

outboxService := domainoutbox.NewService(db)
producer, err := emailoutbox.NewEmailReceivedProducer(ctx, outboxService, emailoutbox.Config{
    KafkaBrokers:    strings.Split(cfg.KafkaBrokers, ","),
    KafkaUsername:   cfg.KafkaSaslUsername,
    KafkaPassword:   cfg.KafkaSaslPassword,
    KafkaMechanism:  cfg.KafkaSaslMechanism,
    KafkaTLSEnabled: cfg.KafkaTlsEnabled,
    RegistryURL:     cfg.SchemaRegistryURL,
    InstanceID:      uuid.NewString(),
    BatchSize:       cfg.BatchSize, MaxAttempts: cfg.MaxAttempts,
    ClaimTTL:        cfg.ClaimTTL, PollInterval: cfg.PollInterval,
})
if err != nil {
    log.Fatal().Err(err).Msg("Failed to create email-received producer")
}
defer producer.Close()

runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
defer stop()

log.Info().Msg("Email-received producer running")
if err := producer.Run(runCtx); err != nil {
    log.Info().Err(err).Msg("Email-received producer stopped")
}
```

`main.go` no longer imports `libs/kafka`, `libs/schemaregistry`, or `libs/eventschemas` at all — topic/subject/schema are fixed inside `emailoutbox.NewEmailReceivedProducer` itself (Section 3), so nothing about `eventschemas.EmailReceived` needs to be named at this call site either. Only `emailoutbox`/`domainoutbox` (for wiring) and the usual `postgres`/`logger`/`uuid`/`signal`/`syscall` imports remain.

**Consumer** (`services/core/cmd/email-received-consumer/main.go`) mirrors this exactly, with `GroupName` added to `Config` and no `Close()` call (per Section 2, `Consumer[T]`'s raw ingestion loop closes its own Kafka client).

## 5. Lifecycle and error handling

- **Construction can now fail on I/O** (`NewFromConfig` calls `kafka.NewClient` and `schemaregistry.NewClient`/`RegisterEncode`/`RegisterDecode`, all of which can error). This isn't new failure surface at the process level — `main.go` already `log.Fatal()`s on exactly these same calls today, just spread across several lines instead of one. `NewFromConfig` closes whatever it already opened before returning an error (see the `kafkaClient.Close()` calls on the producer's error paths) so a failed construction doesn't leak a connection.
- **`Producer[T]` needs an explicit `Close()`** because it owns the Kafka client's lifecycle once built via `NewFromConfig`. A `Producer[T]` built via the low-level `New` (tests) has a no-op `Close`.
- **`Consumer[T]` does not need a `Close()`** — its raw ingestion loop already closes its own Kafka client internally when its polling goroutine exits (existing `KafkaConsumer.Run` behavior, unchanged, `defer c.client.Close()` inside the goroutine).
- **Per-record error handling is unchanged** from the consolidation refactor: a decode/encode/send failure marks that one record failed and the batch continues; a `MarkProcessed`/`MarkFailed` failure itself is logged, not fatal.

## Testing implications

- `libs/outbox`/`libs/inbox` gain their own unit test suites for the `New[T]` constructor path — fetch/process/mark loop resilience, one-bad-record-doesn't-stop-the-batch, decode/encode-failure-marks-failed-and-continues — using hand-rolled fakes (`fakeProducer`/`fakeEncoder`/`fakeRawConsumer`/`fakeDecoder`) and a local test type (`testRecordV1`-shaped, matching the fixture style already established in `libs/schemaregistry/serde_test.go`), not `eventschemas.EmailReceived`. These are the exact tests written for the consolidation refactor's `infra/outbox`/`infra/inbox` packages, moved here and made generic.
- One new integration-style test per package for `NewFromConfig[T]` itself — proves the full build path (real Kafka client, real Apicurio registry, real `RegisterEncode`/`RegisterDecode`) wires correctly end to end. This is new coverage; today nothing automated exercises "build everything from scratch," only the manual smoke test performed during the consolidation refactor.
- `infra/outbox.NewEmailReceivedProducer` carries no logic of its own — pure identity mapping and function-value wiring — so it gets no dedicated test file; the generic engine's tests already cover the loop/error-handling logic, and `domain/outbox`'s own tests cover `FetchBatch`/`MarkProcessed`/`MarkFailed`.
- `infra/inbox.NewEmailReceivedConsumer` carries one real piece of logic — the wire-to-domain-event mapping closure passed as `Deps.Process` (Section 3) — and today's `TestPollOnce_decodesAndProcesses` (written in the consolidation refactor) splits across the boundary this spec introduces:
  - The decode-then-call-Process wiring it exercises (real registry, decode failure → `MarkFailed` + continue, decode success → `Process` called with the decoded value) moves to `libs/inbox`'s own generic tests, using a local test type instead of `eventschemas.EmailReceived`.
  - The six-field mapping itself — the actual thing that was app-specific about the old test — gets a new, small, `-short`-compatible unit test directly in `infra/inbox`: construct the `Process` closure (or drive it through `NewEmailReceivedConsumer` with a mocked `domaininbox.Service`) with a hand-built `eventschemas.EmailReceived`, and assert the domain service receives the correctly-mapped `EmailReceivedEvent`. This needs no live registry — the closure never touches Avro, only struct fields.
- `domain/outbox`, `domain/inbox`: untouched, no test changes.

## Explicitly deferred / out of scope

- Retrofitting a second topic through this shape (this spec only proves the shape works for `email-received`, generalized).
- Any registry/Kafka auth work beyond what exists today.
- Splitting `libs/outbox`/`libs/inbox` into their own repos or publishing as external modules.
- Revisiting the `RawProducer`/`RawConsumer`/`KafkaConsumer`/`NewKafkaProducer`/`NewKafkaConsumer` renames' blast radius outside this repo (none — these are unexported-package-internal or only referenced from the two `infra/` adapters and their tests, all updated in this same change).
