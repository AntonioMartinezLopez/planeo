# Generic Kafka Outbox/Inbox Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Push the email-received-specific produce/consume loops down into generic `libs/outbox.Producer[T]`/`libs/inbox.Consumer[T]` types (Kafka client construction, schema-registry construction/registration, the ticker loop, encode/decode), leaving each service with only a thin, named adapter that fixes topic/subject/schema identity and wires domain-service methods in as function values.

**Architecture:** Two renames (`libs/outbox.Producer`→`RawProducer`, `libs/inbox.Consumer`→`KafkaConsumer`) free up the names for two new generic types. Each gets a two-tier constructor: `New[T]` (low-level, takes pre-built dependencies, no I/O, used by tests) and `NewFromConfig[T]` (high-level, builds Kafka/registry connections itself from flat config, used by production). Per-service `infra/outbox`/`infra/inbox` packages shrink to a type alias plus one constructor each.

**Tech Stack:** Go 1.26.5, generics, `github.com/twmb/franz-go/pkg/{kgo,sr}`, `github.com/iskorotkov/avro/v2`, existing `libs/kafka`/`libs/schemaregistry`/`libs/eventschemas`.

## Global Constraints

- Design spec: `docs/superpowers/specs/2026-07-28-generic-kafka-outbox-inbox-engine-design.md` — read it for full rationale; this plan implements it task-by-task.
- Module: `planeo`, Go `1.26.5`.
- `libs/outbox.BuildConfig`/`libs/inbox.BuildConfig`, and the thin per-service adapters' own `Config`, use only flat scalar fields — no `kafka.Config`, no `schemaregistry.Config`, anywhere on the path from `cmd/main.go` down to `NewFromConfig`. `kafka.Config`/`schemaregistry.Config` are constructed in exactly one place each, inside each package's own `NewFromConfig`.
- Zero change to `domain/outbox`/`domain/inbox` — their existing method signatures already match the required function-value shapes.
- Domain services' per-topic method naming (`ProcessEmailReceived`, not a generic `Process`) is preserved — resolved via Go method-value binding, not interface renaming.
- No placeholder/TODO code — every task below is complete, runnable code.
- Hand-rolled fakes for `RawProducer`/`Encoder[T]`/`RawConsumer`/`Decoder[T]` in tests, not mockery — these are narrow, infra-owned interfaces, matching this codebase's existing convention (mockery is only used for domain-layer interfaces, which are unchanged here).

---

## Task 1: `libs/outbox` — rename raw Producer, add generic `Producer[T]` engine

**Files:**
- Modify: `libs/outbox/producer.go`
- Create: `libs/outbox/generic_producer.go`
- Test: `libs/outbox/generic_producer_test.go`

**Interfaces:**
- Produces: `outbox.RawProducer`, `outbox.NewKafkaProducer(client syncProducer) RawProducer`, `outbox.Encoder[T any]{Encode(v T) ([]byte, error)}`, `outbox.Deps[T]{FetchBatch, MarkProcessed, MarkFailed}`, `outbox.Config{Topic, InstanceID, BatchSize, MaxAttempts, ClaimTTL, PollInterval}`, `outbox.Producer[T]`, `outbox.New[T](rawProducer RawProducer, encoder Encoder[T], deps Deps[T], cfg Config) *Producer[T]`, `outbox.BuildConfig{KafkaBrokers, KafkaUsername, KafkaPassword, KafkaMechanism, KafkaTLSEnabled, RegistryURL, Subject, Schema}`, `outbox.NewFromConfig[T](ctx, build BuildConfig, deps Deps[T], cfg Config) (*Producer[T], error)`, `(*Producer[T]).Run/PollOnce/Close`. Tasks 3 and 5 depend on these exact names.

- [ ] **Step 1: Rename `Producer`→`RawProducer`, `NewProducer`→`NewKafkaProducer`**

Replace the full content of `libs/outbox/producer.go`:

```go
// libs/outbox/producer.go
package outbox

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// RawProducer sends a single record to Kafka. Implemented by kafkaProducer in
// production; test code can supply a fake so an adapter's poll/mark logic is
// unit-testable without a live broker.
type RawProducer interface {
	ProduceSync(ctx context.Context, topic string, key, value []byte) error
}

// syncProducer is the narrow slice of *kgo.Client's API kafkaProducer needs -
// declared here (not imported) so kafkaProducer can be driven by a fake in
// tests without a live broker. *kgo.Client (built via planeo/libs/kafka.NewClient)
// satisfies this structurally; production code never constructs a
// syncProducer value directly, it passes a real *kgo.Client.
type syncProducer interface {
	ProduceSync(ctx context.Context, rs ...*kgo.Record) kgo.ProduceResults
}

type kafkaProducer struct {
	client syncProducer
}

func (p *kafkaProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	result := p.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: key, Value: value})
	return result.FirstErr()
}

// NewKafkaProducer wraps an already-authenticated *kgo.Client (see
// planeo/libs/kafka.NewClient) as a RawProducer. This package no longer
// constructs or owns the client's lifecycle - the caller does, and is
// responsible for calling client.Close() on shutdown.
func NewKafkaProducer(client syncProducer) RawProducer {
	return &kafkaProducer{client: client}
}
```

- [ ] **Step 2: Verify `libs/outbox` still builds**

Run: `go build ./libs/outbox/...`
Expected: exits 0. (The rest of the module — `services/email/internal/infra/outbox` and `services/email/cmd/email-received-producer` — will now fail to build, since they still reference the old `Producer`/`NewProducer` names. This is expected and gets fixed in Task 3; do not fix it here.)

- [ ] **Step 3: Write the failing test for the generic engine**

```go
// libs/outbox/generic_producer_test.go
package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"planeo/libs/outbox"
	"planeo/libs/schemaregistry"

	avro "github.com/iskorotkov/avro/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testWire struct {
	Name string `avro:"name" json:"name"`
}

const testWireSchemaRaw = `{
  "type": "record", "name": "TestWire", "namespace": "planeo.test",
  "fields": [{"name": "name", "type": "string"}]
}`

var testWireSchema = avro.MustParse(testWireSchemaRaw)

// fakeService tracks calls made through Deps[T]'s function values — a
// hand-rolled fake, not a mockery mock, since Deps[T] is a struct of plain
// functions rather than an interface mockery can generate against.
type fakeService struct {
	fetchBatchReturn   []outbox.Record
	fetchBatchErr      error
	markProcessedCalls []int64
	markFailedCalls    []markFailedCall
}

type markFailedCall struct {
	id          int64
	err         error
	maxAttempts int
}

func (f *fakeService) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]outbox.Record, error) {
	return f.fetchBatchReturn, f.fetchBatchErr
}

func (f *fakeService) MarkProcessed(ctx context.Context, id int64) error {
	f.markProcessedCalls = append(f.markProcessedCalls, id)
	return nil
}

func (f *fakeService) MarkFailed(ctx context.Context, id int64, err error, maxAttempts int) error {
	f.markFailedCalls = append(f.markFailedCalls, markFailedCall{id, err, maxAttempts})
	return nil
}

// fakeRawProducer is a hand-rolled RawProducer double. failOnCall, if
// non-zero, makes only the Nth call (1-indexed) fail — lets a single fake
// exercise "one record in a batch fails, the rest succeed".
type fakeRawProducer struct {
	gotTopic       string
	gotKey, gotVal []byte
	err            error
	failOnCall     int
	calls          int
}

func (f *fakeRawProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	f.calls++
	f.gotTopic, f.gotKey, f.gotVal = topic, key, value
	if f.failOnCall == 0 || f.calls == f.failOnCall {
		return f.err
	}
	return nil
}

type fakeEncoder struct {
	err error
}

func (f *fakeEncoder) Encode(v testWire) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte("encoded"), nil
}

func testConfig() outbox.Config {
	return outbox.Config{
		Topic: "test-topic", InstanceID: "instance-a",
		BatchSize: 10, MaxAttempts: 5, ClaimTTL: 30 * time.Second,
	}
}

func TestProducer(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("propagates a FetchBatch error without processing any record", func(t *testing.T) {
		fetchErr := errors.New("fetch failed")
		fake := &fakeService{fetchBatchErr: fetchErr}

		producer := outbox.New[testWire](
			&fakeRawProducer{}, &fakeEncoder{},
			outbox.Deps[testWire]{FetchBatch: fake.FetchBatch, MarkProcessed: fake.MarkProcessed, MarkFailed: fake.MarkFailed},
			testConfig(),
		)

		err := producer.PollOnce(context.Background())

		if !errors.Is(err, fetchErr) {
			t.Errorf("got err %v, want %v", err, fetchErr)
		}
	})

	t.Run("processes every fetched record, even if one fails to send", func(t *testing.T) {
		okPayload, err := json.Marshal(testWire{Name: "ok"})
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		records := []outbox.Record{
			{ID: 1, Topic: "test-topic", Payload: okPayload},
			{ID: 2, Topic: "test-topic", Payload: okPayload},
		}
		fake := &fakeService{fetchBatchReturn: records}
		rawProducer := &fakeRawProducer{err: errors.New("boom"), failOnCall: 1}

		producer := outbox.New[testWire](
			rawProducer, &fakeEncoder{},
			outbox.Deps[testWire]{FetchBatch: fake.FetchBatch, MarkProcessed: fake.MarkProcessed, MarkFailed: fake.MarkFailed},
			testConfig(),
		)

		if err := producer.PollOnce(context.Background()); err != nil {
			t.Fatalf("PollOnce: %v", err)
		}
		if len(fake.markFailedCalls) != 1 || fake.markFailedCalls[0].id != 1 {
			t.Errorf("got markFailedCalls %+v, want exactly one call for id 1", fake.markFailedCalls)
		}
		if len(fake.markProcessedCalls) != 1 || fake.markProcessedCalls[0] != 2 {
			t.Errorf("got markProcessedCalls %+v, want exactly one call for id 2", fake.markProcessedCalls)
		}
	})

	t.Run("marks a record failed and continues when its payload is not valid JSON", func(t *testing.T) {
		badRecord := outbox.Record{ID: 1, Topic: "test-topic", Payload: []byte("not-json")}
		fake := &fakeService{fetchBatchReturn: []outbox.Record{badRecord}}

		producer := outbox.New[testWire](
			&fakeRawProducer{}, &fakeEncoder{},
			outbox.Deps[testWire]{FetchBatch: fake.FetchBatch, MarkProcessed: fake.MarkProcessed, MarkFailed: fake.MarkFailed},
			testConfig(),
		)

		if err := producer.PollOnce(context.Background()); err != nil {
			t.Fatalf("PollOnce should not abort the batch on a bad payload, got: %v", err)
		}
		if len(fake.markFailedCalls) != 1 || fake.markFailedCalls[0].id != 1 {
			t.Errorf("got markFailedCalls %+v, want exactly one call for id 1", fake.markFailedCalls)
		}
	})
}

func startRegistry(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "apicurio/apicurio-registry:3.3.0",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/apis/ccompat/v7/subjects").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start apicurio container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Errorf("failed to terminate apicurio container: %v", err)
		}
	})
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}
	return "http://" + host + ":" + port.Port() + "/apis/ccompat/v7"
}

// TestNewFromConfig_wiresCorrectly proves NewFromConfig's full build path —
// real Kafka client construction, real schema-registry client +
// RegisterEncode against a live Apicurio — succeeds and returns a working
// Producer[T]. kgo.NewClient connects to brokers lazily (only on first
// actual request — confirmed via `go doc github.com/twmb/franz-go/pkg/kgo
// NewClient`), so a real Kafka broker isn't needed here to prove the wiring
// succeeds; TestPollOnce_encodesAndSends below is what proves genuine
// end-to-end production, using the low-level New with a real Encoder and a
// fake raw producer instead.
func TestNewFromConfig_wiresCorrectly(t *testing.T) {
	registryURL := startRegistry(t)

	fake := &fakeService{}
	producer, err := outbox.NewFromConfig[testWire](context.Background(),
		outbox.BuildConfig{
			KafkaBrokers: []string{"localhost:0"},
			RegistryURL:  registryURL,
			Subject:      "test-wire-value",
			Schema:       testWireSchema,
		},
		outbox.Deps[testWire]{FetchBatch: fake.FetchBatch, MarkProcessed: fake.MarkProcessed, MarkFailed: fake.MarkFailed},
		testConfig(),
	)
	if err != nil {
		t.Fatalf("NewFromConfig: %v", err)
	}
	if producer == nil {
		t.Fatal("NewFromConfig returned a nil producer with a nil error")
	}
	if err := producer.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestPollOnce_encodesAndSends proves the bytes handed to the raw producer
// are genuinely Avro-encoded (via a real registry), not just passed
// through — the one test in this file that needs a live Apicurio container
// for the encode step itself.
func TestPollOnce_encodesAndSends(t *testing.T) {
	registryURL := startRegistry(t)
	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: registryURL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[testWire](context.Background(), serde, "test-wire-value", testWireSchema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}
	encoder := schemaregistry.NewEncoder[testWire](serde)

	wire := testWire{Name: "hi"}
	jsonPayload, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	record := outbox.Record{ID: 1, Topic: "test-topic", Key: []byte("1"), Payload: jsonPayload}

	fakeRP := &fakeRawProducer{}
	fake := &fakeService{fetchBatchReturn: []outbox.Record{record}}

	producer := outbox.New[testWire](
		fakeRP, encoder,
		outbox.Deps[testWire]{FetchBatch: fake.FetchBatch, MarkProcessed: fake.MarkProcessed, MarkFailed: fake.MarkFailed},
		testConfig(),
	)

	if err := producer.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	if fakeRP.gotTopic != "test-topic" {
		t.Errorf("got topic %q, want %q", fakeRP.gotTopic, "test-topic")
	}
	if string(fakeRP.gotKey) != "1" {
		t.Errorf("got key %q, want %q", fakeRP.gotKey, "1")
	}

	var decoded testWire
	if err := schemaregistry.Decode(context.Background(), serde, fakeRP.gotVal, testWireSchema, &decoded); err != nil {
		t.Fatalf("decoding what was sent: %v", err)
	}
	if decoded.Name != "hi" {
		t.Errorf("got Name=%q, want %q", decoded.Name, "hi")
	}
	if len(fake.markProcessedCalls) != 1 || fake.markProcessedCalls[0] != 1 {
		t.Errorf("got markProcessedCalls %+v, want exactly one call for id 1", fake.markProcessedCalls)
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `go test ./libs/outbox/... -v`
Expected: FAIL to compile — `outbox.New`, `outbox.Deps`, `outbox.Config`, `outbox.BuildConfig`, `outbox.NewFromConfig` don't exist yet.

- [ ] **Step 5: Write `generic_producer.go`**

```go
// libs/outbox/generic_producer.go
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
	Topic, InstanceID      string
	BatchSize, MaxAttempts int
	ClaimTTL, PollInterval time.Duration
}

// Producer[T] owns a full outbox produce loop for one topic and one wire
// type T: poll the domain service for a batch of pending rows, JSON-decode
// each row's staged payload into T, Avro-encode it, send it, and record the
// outcome. Zero knowledge of what T actually is beyond its JSON/Avro tags.
type Producer[T any] struct {
	rawProducer  RawProducer
	encoder      Encoder[T]
	service      Deps[T]
	cfg          Config
	closeFn      func() error // no-op unless built via NewFromConfig
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
// wrap one or five values in a struct. This package still imports both
// internally (it has to, to actually call
// kafka.NewClient/schemaregistry.NewClient); this only decouples the
// caller's Config shape from theirs.
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

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./libs/outbox/... -v -short` (requires Docker regardless — `TestNewFromConfig_wiresCorrectly`/`TestPollOnce_encodesAndSends` have no `testing.Short()` guard, matching this codebase's existing convention where `-short` selects *in* the fake-based tests via `if !testing.Short() { t.Skip() }`, rather than skipping Docker-based ones; see e.g. the already-committed `services/email/internal/infra/outbox/email_received_producer_test.go` before this task's Step 2 deletes it)
Expected: PASS for `TestProducer` (and its three subtests), `TestNewFromConfig_wiresCorrectly`, and `TestPollOnce_encodesAndSends` — all four run under `-short` in this codebase's convention.

- [ ] **Step 7: Format and commit**

```bash
gofmt -w libs/outbox/producer.go libs/outbox/generic_producer.go libs/outbox/generic_producer_test.go
git add libs/outbox
git commit -m "Add libs/outbox.Producer[T]: generic produce/encode/mark engine, rename raw Producer to RawProducer"
```

---

## Task 2: `libs/inbox` — rename raw Consumer, add generic `Consumer[T]` engine

**Files:**
- Modify: `libs/inbox/consumer.go`
- Create: `libs/inbox/generic_consumer.go`
- Test: `libs/inbox/generic_consumer_test.go`

**Interfaces:**
- Produces: `inbox.KafkaConsumer`, `inbox.NewKafkaConsumer(client fetchClient, topic string, store saver) *KafkaConsumer`, `inbox.RawConsumer{Run(ctx) error}`, `inbox.Decoder[T any]{Decode(ctx, []byte) (T, error)}`, `inbox.Deps[T]{FetchBatch, Process, MarkFailed}`, `inbox.Config{Topic, InstanceID, BatchSize, MaxAttempts, ClaimTTL, PollInterval}`, `inbox.Consumer[T]`, `inbox.New[T](rawConsumer RawConsumer, decoder Decoder[T], deps Deps[T], cfg Config) *Consumer[T]`, `inbox.SaveFunc` (+ its `Save` method), `inbox.BuildConfig{KafkaBrokers, KafkaUsername, KafkaPassword, KafkaMechanism, KafkaTLSEnabled, GroupName, RegistryURL, Subject, Schema}`, `inbox.NewFromConfig[T](ctx, build BuildConfig, save SaveFunc, deps Deps[T], cfg Config) (*Consumer[T], error)`, `(*Consumer[T]).Run/PollOnce`. Tasks 4 and 5 depend on these exact names.

- [ ] **Step 1: Rename `Consumer`→`KafkaConsumer`, `NewConsumer`→`NewKafkaConsumer`, add `RawConsumer`**

Replace the full content of `libs/inbox/consumer.go`:

```go
// libs/inbox/consumer.go
package inbox

import (
	"context"

	"planeo/libs/logger"

	"github.com/twmb/franz-go/pkg/kgo"
)

// RawConsumer starts the raw Kafka-to-inbox-table ingestion loop. Declared
// here so the generic Consumer[T] (generic_consumer.go) can be driven by a
// fake in tests without a live broker; *KafkaConsumer satisfies this
// structurally, no wrapper needed.
type RawConsumer interface {
	Run(ctx context.Context) error
}

type saver interface {
	Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (inserted bool, err error)
}

// fetchClient is the narrow slice of *kgo.Client's API KafkaConsumer needs -
// declared here (not imported) so Run can be driven by a fake in tests
// without a live broker. *kgo.Client (built via planeo/libs/kafka.NewClient)
// satisfies this structurally; production code never constructs a
// fetchClient value directly, it passes a real *kgo.Client.
type fetchClient interface {
	PollFetches(ctx context.Context) kgo.Fetches
	CommitRecords(ctx context.Context, rs ...*kgo.Record) error
	Close()
}

// KafkaConsumer reads from Kafka and persists into the inbox, committing the
// offset only after Save succeeds. No Handler is invoked here. The given
// client must already be configured with its consumer group and topic
// subscription (via kgo.ConsumerGroup/kgo.ConsumeTopics passed as extraOpts
// to planeo/libs/kafka.NewClient) - KafkaConsumer only drives it, it doesn't
// configure it.
type KafkaConsumer struct {
	client fetchClient
	topic  string
	store  saver
}

func NewKafkaConsumer(client fetchClient, topic string, store saver) *KafkaConsumer {
	return &KafkaConsumer{
		client: client,
		topic:  topic,
		store:  store,
	}
}

// Run subscribes to the configured topic and persists each fetched record
// into the inbox, deduped on the Kafka coordinate, committing the offset
// only after a successful Save. Non-blocking — starts its own background
// goroutine and returns immediately.
func (c *KafkaConsumer) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)

	go func() {
		defer c.client.Close()

		for {
			if ctx.Err() != nil {
				return
			}

			fetches := c.client.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return
			}

			fetches.EachError(func(_ string, _ int32, err error) {
				log.Error().Err(err).Msg("kafka fetch error")
			})

			// Kafka offset commits are a single cumulative per-partition
			// marker, not a per-record acknowledgement: if record N's Save
			// fails but record N+1's Save (and commit) succeeds, the
			// committed offset would advance past N, and N would never be
			// redelivered — a silent, permanent loss of exactly the message
			// this pattern exists to protect. To prevent that, once a
			// partition has a failure in this poll, stop committing further
			// records on that partition for the rest of the poll (Save's
			// (topic, partition, offset) dedup makes it safe to reprocess
			// already-saved records on the next poll/redelivery).
			failedPartitions := map[int32]bool{}
			fetches.EachRecord(func(record *kgo.Record) {
				if failedPartitions[record.Partition] {
					return
				}

				if _, err := c.store.Save(ctx, c.topic, record.Partition, record.Offset, record.Value); err != nil {
					log.Error().Err(err).Int32("partition", record.Partition).Int64("offset", record.Offset).Msg("failed to persist inbox record, halting commits on this partition until next poll")
					failedPartitions[record.Partition] = true
					return
				}

				if err := c.client.CommitRecords(ctx, record); err != nil {
					log.Error().Err(err).Int32("partition", record.Partition).Int64("offset", record.Offset).Msg("failed to commit kafka offset, halting further commits on this partition")
					failedPartitions[record.Partition] = true
				}
			})
		}
	}()

	return nil
}
```

- [ ] **Step 2: Verify `libs/inbox` still builds**

Run: `go build ./libs/inbox/...`
Expected: exits 0. (`services/core/internal/infra/inbox` and `services/core/cmd/email-received-consumer` will now fail to build — expected, fixed in Task 4.)

- [ ] **Step 3: Write the failing test for the generic engine**

```go
// libs/inbox/generic_consumer_test.go
package inbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"planeo/libs/inbox"
	"planeo/libs/schemaregistry"

	avro "github.com/iskorotkov/avro/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testWire struct {
	Name string `avro:"name" json:"name"`
}

const testWireSchemaRaw = `{
  "type": "record", "name": "TestWire", "namespace": "planeo.test",
  "fields": [{"name": "name", "type": "string"}]
}`

var testWireSchema = avro.MustParse(testWireSchemaRaw)

// fakeKafkaConsumer is a hand-rolled RawConsumer double — a single-method
// startup interface, not worth mockery generation.
type fakeKafkaConsumer struct {
	err   error
	calls int
}

func (f *fakeKafkaConsumer) Run(ctx context.Context) error {
	f.calls++
	return f.err
}

// fakeService tracks calls made through Deps[T]'s function values — a
// hand-rolled fake, not a mockery mock, since Deps[T] is a struct of plain
// functions rather than an interface mockery can generate against.
type fakeService struct {
	fetchBatchReturn []inbox.Record
	fetchBatchErr    error
	processCalls     []processCall
	processErr       error
	markFailedCalls  []markFailedCall
}

type processCall struct {
	id          int64
	event       testWire
	maxAttempts int
}

type markFailedCall struct {
	id          int64
	err         error
	maxAttempts int
}

func (f *fakeService) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]inbox.Record, error) {
	return f.fetchBatchReturn, f.fetchBatchErr
}

func (f *fakeService) Process(ctx context.Context, id int64, event testWire, maxAttempts int) error {
	f.processCalls = append(f.processCalls, processCall{id, event, maxAttempts})
	return f.processErr
}

func (f *fakeService) MarkFailed(ctx context.Context, id int64, err error, maxAttempts int) error {
	f.markFailedCalls = append(f.markFailedCalls, markFailedCall{id, err, maxAttempts})
	return nil
}

// fakeDecoder stands in for a real Avro decoder in tests that only care
// about the decode-failure path — TestPollOnce_decodesAndProcesses below is
// the one test that needs a real, registry-backed decoder.
type fakeDecoder struct {
	err error
}

func (f *fakeDecoder) Decode(ctx context.Context, b []byte) (testWire, error) {
	if f.err != nil {
		return testWire{}, f.err
	}
	return testWire{Name: "decoded"}, nil
}

func testConfig() inbox.Config {
	return inbox.Config{
		Topic: "test-topic", InstanceID: "instance-a",
		BatchSize: 10, MaxAttempts: 5, ClaimTTL: 30 * time.Second,
	}
}

func TestConsumer(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("propagates a FetchBatch error without processing any record", func(t *testing.T) {
		fetchErr := errors.New("fetch failed")
		fake := &fakeService{fetchBatchErr: fetchErr}

		consumer := inbox.New[testWire](
			&fakeKafkaConsumer{}, &fakeDecoder{},
			inbox.Deps[testWire]{FetchBatch: fake.FetchBatch, Process: fake.Process, MarkFailed: fake.MarkFailed},
			testConfig(),
		)

		err := consumer.PollOnce(context.Background())

		if !errors.Is(err, fetchErr) {
			t.Errorf("got err %v, want %v", err, fetchErr)
		}
	})

	t.Run("marks a record failed and continues when decoding fails", func(t *testing.T) {
		fake := &fakeService{fetchBatchReturn: []inbox.Record{{ID: 1, Topic: "test-topic", Payload: []byte("bad")}}}

		consumer := inbox.New[testWire](
			&fakeKafkaConsumer{}, &fakeDecoder{err: errors.New("boom")},
			inbox.Deps[testWire]{FetchBatch: fake.FetchBatch, Process: fake.Process, MarkFailed: fake.MarkFailed},
			testConfig(),
		)

		if err := consumer.PollOnce(context.Background()); err != nil {
			t.Fatalf("PollOnce should not abort the batch on a decode failure, got: %v", err)
		}
		if len(fake.markFailedCalls) != 1 || fake.markFailedCalls[0].id != 1 {
			t.Errorf("got markFailedCalls %+v, want exactly one call for id 1", fake.markFailedCalls)
		}
		if len(fake.processCalls) != 0 {
			t.Errorf("got processCalls %+v, want none", fake.processCalls)
		}
	})

	t.Run("decodes and processes a record", func(t *testing.T) {
		fake := &fakeService{fetchBatchReturn: []inbox.Record{{ID: 1, Topic: "test-topic", Payload: []byte("anything")}}}

		consumer := inbox.New[testWire](
			&fakeKafkaConsumer{}, &fakeDecoder{},
			inbox.Deps[testWire]{FetchBatch: fake.FetchBatch, Process: fake.Process, MarkFailed: fake.MarkFailed},
			testConfig(),
		)

		if err := consumer.PollOnce(context.Background()); err != nil {
			t.Fatalf("PollOnce: %v", err)
		}
		if len(fake.processCalls) != 1 || fake.processCalls[0].id != 1 || fake.processCalls[0].event.Name != "decoded" {
			t.Errorf("got processCalls %+v, want one call for id 1 with event.Name=decoded", fake.processCalls)
		}
	})

	t.Run("Run starts the raw Kafka consumer and propagates its startup error", func(t *testing.T) {
		fake := &fakeService{}
		kafkaErr := errors.New("broker unreachable")
		fakeConsumer := &fakeKafkaConsumer{err: kafkaErr}

		consumer := inbox.New[testWire](
			fakeConsumer, &fakeDecoder{},
			inbox.Deps[testWire]{FetchBatch: fake.FetchBatch, Process: fake.Process, MarkFailed: fake.MarkFailed},
			testConfig(),
		)

		err := consumer.Run(context.Background())

		if !errors.Is(err, kafkaErr) {
			t.Errorf("got err %v, want %v", err, kafkaErr)
		}
		if fakeConsumer.calls != 1 {
			t.Errorf("got %d calls to the raw consumer, want 1", fakeConsumer.calls)
		}
	})
}

func startRegistry(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "apicurio/apicurio-registry:3.3.0",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/apis/ccompat/v7/subjects").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start apicurio container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Errorf("failed to terminate apicurio container: %v", err)
		}
	})
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}
	return "http://" + host + ":" + port.Port() + "/apis/ccompat/v7"
}

// TestNewFromConfig_wiresCorrectly proves NewFromConfig's full build path —
// real Kafka client construction, real schema-registry client +
// RegisterDecode against a live Apicurio — succeeds and returns a working
// Consumer[T]. kgo.NewClient connects to brokers lazily, so a real Kafka
// broker isn't needed to prove the wiring succeeds.
func TestNewFromConfig_wiresCorrectly(t *testing.T) {
	registryURL := startRegistry(t)

	// The registry must already have this subject registered for
	// RegisterDecode to succeed (it's lookup-only, non-creating — see
	// libs/schemaregistry's own tests). Register it here first, the way
	// cmd/registerschemas would in production.
	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: registryURL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := schemaregistry.Register(context.Background(), client, "test-wire-value", testWireSchema); err != nil {
		t.Fatalf("Register: %v", err)
	}

	fake := &fakeService{}
	consumer, err := inbox.NewFromConfig[testWire](context.Background(),
		inbox.BuildConfig{
			KafkaBrokers: []string{"localhost:0"},
			GroupName:    "test-group",
			RegistryURL:  registryURL,
			Subject:      "test-wire-value",
			Schema:       testWireSchema,
		},
		func(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error) {
			return true, nil
		},
		inbox.Deps[testWire]{FetchBatch: fake.FetchBatch, Process: fake.Process, MarkFailed: fake.MarkFailed},
		testConfig(),
	)
	if err != nil {
		t.Fatalf("NewFromConfig: %v", err)
	}
	if consumer == nil {
		t.Fatal("NewFromConfig returned a nil consumer with a nil error")
	}
}

// TestPollOnce_decodesAndProcesses proves the event handed to Process is
// genuinely Avro-decoded (via a real registry), not just passed through.
func TestPollOnce_decodesAndProcesses(t *testing.T) {
	registryURL := startRegistry(t)
	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: registryURL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[testWire](context.Background(), serde, "test-wire-value", testWireSchema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}
	decoder := schemaregistry.NewDecoder[testWire](serde, testWireSchema)

	wire := testWire{Name: "hi"}
	encoded, err := schemaregistry.Encode(serde, wire)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	fake := &fakeService{fetchBatchReturn: []inbox.Record{{ID: 1, Topic: "test-topic", Payload: encoded}}}

	consumer := inbox.New[testWire](
		&fakeKafkaConsumer{}, decoder,
		inbox.Deps[testWire]{FetchBatch: fake.FetchBatch, Process: fake.Process, MarkFailed: fake.MarkFailed},
		testConfig(),
	)

	if err := consumer.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
	if len(fake.processCalls) != 1 || fake.processCalls[0].event.Name != "hi" {
		t.Errorf("got processCalls %+v, want one call with event.Name=hi", fake.processCalls)
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `go test ./libs/inbox/... -v`
Expected: FAIL to compile — `inbox.New`, `inbox.Deps`, `inbox.Config`, `inbox.SaveFunc`, `inbox.BuildConfig`, `inbox.NewFromConfig` don't exist yet.

- [ ] **Step 5: Write `generic_consumer.go`**

```go
// libs/inbox/generic_consumer.go
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
	Topic, InstanceID      string
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
// scalar fields only, for the same reason as libs/outbox.BuildConfig.
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

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./libs/inbox/... -v -short` (requires Docker regardless — see Task 1 Step 6's note on this codebase's `-short` convention)
Expected: PASS for `TestConsumer` (and its four subtests), `TestNewFromConfig_wiresCorrectly`, and `TestPollOnce_decodesAndProcesses` — all run under `-short`.

- [ ] **Step 7: Format and commit**

```bash
gofmt -w libs/inbox/consumer.go libs/inbox/generic_consumer.go libs/inbox/generic_consumer_test.go
git add libs/inbox
git commit -m "Add libs/inbox.Consumer[T]: generic ingest/decode/process engine, rename raw Consumer to KafkaConsumer"
```

---

## Task 3: Producer side — thin adapter + `main.go`

**Files:**
- Modify: `services/email/internal/infra/outbox/email_received_producer.go`
- Delete: `services/email/internal/infra/outbox/email_received_producer_test.go`
- Modify: `services/email/cmd/email-received-producer/main.go`

**Interfaces:**
- Consumes: `libsoutbox.Producer[T]`, `libsoutbox.Deps[T]`, `libsoutbox.Config`, `libsoutbox.BuildConfig`, `libsoutbox.NewFromConfig[T]` (Task 1); `domainoutbox.Service.FetchBatch/MarkProcessed/MarkFailed` (unchanged).
- Produces: `outbox.EmailReceivedProducer` (type alias), `outbox.Config{KafkaBrokers, KafkaUsername, KafkaPassword, KafkaMechanism, KafkaTLSEnabled, RegistryURL, InstanceID, BatchSize, MaxAttempts, ClaimTTL, PollInterval}`, `outbox.NewEmailReceivedProducer(ctx, service domainoutbox.Service, cfg Config) (*EmailReceivedProducer, error)`.

- [ ] **Step 1: Rewrite the thin adapter**

Replace the full content of `services/email/internal/infra/outbox/email_received_producer.go`:

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

// EmailReceivedProducer is libs/outbox's generic engine, fixed to the
// email-received topic/subject/schema. This file's only job is that
// fixing: no loop, no encode logic, no Kafka/registry construction of its
// own — all of that lives in libs/outbox.Producer[T].
type EmailReceivedProducer = libsoutbox.Producer[eventschemas.EmailReceived]

// Config is flat all the way through — no kafka.Config, no
// schemaregistry.Config — so this package, and cmd/main.go one level up,
// need neither import just to build one.
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

- [ ] **Step 2: Delete the old test file**

This adapter now carries no logic of its own — pure identity mapping and function-value wiring. `libs/outbox`'s own tests (Task 1) already cover the loop/error-handling logic, and `domain/outbox`'s existing tests cover `FetchBatch`/`MarkProcessed`/`MarkFailed`. There is nothing left here worth a dedicated test file.

```bash
git rm services/email/internal/infra/outbox/email_received_producer_test.go
```

- [ ] **Step 3: Verify the adapter builds**

Run: `go build ./services/email/internal/infra/outbox/...`
Expected: exits 0.

- [ ] **Step 4: Rewrite `main.go`**

Replace the full content of `services/email/cmd/email-received-producer/main.go`:

```go
// services/email/cmd/email-received-producer/main.go
package main

import (
	"context"
	"os/signal"
	domainoutbox "planeo/services/email/internal/domain/outbox"
	emailoutbox "planeo/services/email/internal/infra/outbox"
	"planeo/services/email/internal/infra/postgres"
	"strings"
	"syscall"

	"planeo/libs/logger"

	"github.com/google/uuid"
)

func main() {
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	log := logger.New("email-received-producer")
	ctx := logger.WithContext(context.Background(), log)

	log.Info().Msg("Loading environment variables")
	cfg := LoadConfig(ctx)

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
		ClaimTTL: cfg.ClaimTTL, PollInterval: cfg.PollInterval,
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
}
```

- [ ] **Step 5: Verify the whole email service builds**

Run: `go build ./services/email/...`
Expected: exits 0. This is the first point since Task 1 where `services/email` compiles cleanly again.

- [ ] **Step 6: Run the email service's unit and integration tests**

Run: `task test:email:unit`
Expected: all PASS.

Run: `task test:email:integration`
Expected: all PASS (requires Docker).

- [ ] **Step 7: Commit**

```bash
gofmt -w services/email/internal/infra/outbox/email_received_producer.go services/email/cmd/email-received-producer/main.go
git add services/email/internal/infra/outbox services/email/cmd/email-received-producer
git commit -m "Rewire email-received producer onto libs/outbox.Producer[T]"
```

---

## Task 4: Consumer side — thin adapter + `main.go`

**Files:**
- Modify: `services/core/internal/infra/inbox/email_received_consumer.go`
- Modify: `services/core/internal/infra/inbox/email_received_consumer_test.go`
- Modify: `services/core/cmd/email-received-consumer/main.go`

**Interfaces:**
- Consumes: `libsinbox.Consumer[T]`, `libsinbox.Deps[T]`, `libsinbox.Config`, `libsinbox.BuildConfig`, `libsinbox.NewFromConfig[T]`, `libsinbox.SaveFunc` (Task 2); `domaininbox.Service.Save/FetchBatch/ProcessEmailReceived/MarkFailed` (unchanged); `domaininbox.EmailReceivedEvent` (unchanged).
- Produces: `inbox.EmailReceivedConsumer` (type alias), `inbox.Config{KafkaBrokers, KafkaUsername, KafkaPassword, KafkaMechanism, KafkaTLSEnabled, GroupName, RegistryURL, InstanceID, BatchSize, MaxAttempts, ClaimTTL, PollInterval}`, `inbox.NewEmailReceivedConsumer(ctx, service domaininbox.Service, cfg Config) (*EmailReceivedConsumer, error)`, `inbox.mapEmailReceivedEvent(wire eventschemas.EmailReceived) domaininbox.EmailReceivedEvent` (unexported, package-internal).

- [ ] **Step 1: Write the failing test for `mapEmailReceivedEvent`**

This is the one real piece of logic the thin adapter carries — the six-field wire-to-domain-event mapping. It's a pure function, so it's tested directly, with no live registry and no mocked service needed. Replace the full content of `services/core/internal/infra/inbox/email_received_consumer_test.go`:

```go
// services/core/internal/infra/inbox/email_received_consumer_test.go
package inbox_test

import (
	"testing"
	"time"

	"planeo/libs/eventschemas"
	domaininbox "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/infra/inbox"

	"github.com/stretchr/testify/assert"
)

func TestMapEmailReceivedEvent(t *testing.T) {
	wire := eventschemas.EmailReceived{
		Subject:        "hi",
		Body:           "body",
		From:           "a@b.com",
		Date:           time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MessageID:      "msg-1",
		OrganizationID: 7,
	}

	got := inbox.MapEmailReceivedEvent(wire)

	want := domaininbox.EmailReceivedEvent{
		Subject:        "hi",
		Body:           "body",
		From:           "a@b.com",
		Date:           time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MessageID:      "msg-1",
		OrganizationID: 7,
	}
	assert.Equal(t, want, got)
}
```

Note: this test lives in the external `inbox_test` package (matching this codebase's existing convention throughout), so `mapEmailReceivedEvent` must be exported as `MapEmailReceivedEvent` to be reachable from it — a deliberate, small deviation from the design spec's unexported naming, made for testability. It's still package-internal in every practical sense: nothing outside `services/core/internal/infra/inbox` has a reason to call it.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./services/core/internal/infra/inbox/... -v`
Expected: FAIL to compile — `inbox.MapEmailReceivedEvent`, and the new `inbox.Config`/`inbox.NewEmailReceivedConsumer` shapes, don't exist yet (the file still has the pre-Task-2 shape at this point).

- [ ] **Step 3: Rewrite the thin adapter**

Replace the full content of `services/core/internal/infra/inbox/email_received_consumer.go`:

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

// EmailReceivedConsumer is libs/inbox's generic engine, fixed to the
// email-received topic/subject/schema. This file's only two jobs are that
// fixing, and mapping the Avro-decoded wire type into the domain-owned
// event type ProcessEmailReceived expects — no loop, no decode logic, no
// Kafka/registry construction of its own.
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
			Process: func(ctx context.Context, id int64, wire eventschemas.EmailReceived, maxAttempts int) error {
				return service.ProcessEmailReceived(ctx, id, MapEmailReceivedEvent(wire), maxAttempts)
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

// MapEmailReceivedEvent converts the Avro-decoded wire type into the
// domain-owned event type ProcessEmailReceived expects — not
// eventschemas.EmailReceived, the wire type Consumer[T] decodes into, by
// the original Kafka spec's own deliberate design (keep domain/inbox free
// of wire-format types). This six-field mapping is the one piece of real
// logic this adapter carries. Exported (rather than left as an inline
// closure, or unexported) so it's directly testable without constructing a
// full Consumer[T] — which would otherwise require a live schema registry
// just to exercise a struct copy.
func MapEmailReceivedEvent(wire eventschemas.EmailReceived) domaininbox.EmailReceivedEvent {
	return domaininbox.EmailReceivedEvent{
		Subject:        wire.Subject,
		Body:           wire.Body,
		From:           wire.From,
		Date:           wire.Date,
		MessageID:      wire.MessageID,
		OrganizationID: wire.OrganizationID,
	}
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./services/core/internal/infra/inbox/... -v`
Expected: PASS for `TestMapEmailReceivedEvent`.

- [ ] **Step 5: Rewrite `main.go`**

Replace the full content of `services/core/cmd/email-received-consumer/main.go`:

```go
// services/core/cmd/email-received-consumer/main.go
package main

import (
	"context"
	"os/signal"
	"planeo/services/core/internal/domain/category"
	domaininbox "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/domain/request"
	coreinbox "planeo/services/core/internal/infra/inbox"
	"planeo/services/core/internal/infra/llm"
	"planeo/services/core/internal/infra/postgres"
	"strings"
	"syscall"

	"planeo/libs/logger"

	"github.com/google/uuid"
)

func main() {
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	log := logger.New("email-received-consumer")
	ctx := logger.WithContext(context.Background(), log)

	log.Info().Msg("Loading environment variables")
	cfg := LoadConfig(ctx)

	db := postgres.NewClient(ctx, cfg.DatabaseConfig())
	defer db.Close()

	categoryService := category.NewService(db)
	requestService := request.NewService(db)

	llmClient, err := llm.NewClient()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create LLM client")
	}

	inboxService := domaininbox.NewService(db, requestService, categoryService, llmClient)

	consumer, err := coreinbox.NewEmailReceivedConsumer(ctx, inboxService, coreinbox.Config{
		KafkaBrokers:    strings.Split(cfg.KafkaBrokers, ","),
		KafkaUsername:   cfg.KafkaSaslUsername,
		KafkaPassword:   cfg.KafkaSaslPassword,
		KafkaMechanism:  cfg.KafkaSaslMechanism,
		KafkaTLSEnabled: cfg.KafkaTlsEnabled,
		GroupName:       cfg.GroupName,
		RegistryURL:     cfg.SchemaRegistryURL,
		InstanceID:      uuid.NewString(),
		BatchSize:       cfg.BatchSize, MaxAttempts: cfg.MaxAttempts,
		ClaimTTL: cfg.ClaimTTL, PollInterval: cfg.PollInterval,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create email-received consumer")
	}

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Email-received consumer running")
	if err := consumer.Run(runCtx); err != nil {
		log.Info().Err(err).Msg("Email-received consumer stopped")
	}
}
```

- [ ] **Step 6: Verify the whole module builds**

Run: `go build ./...`
Expected: exits 0. This is the first point since Task 1 where the entire module compiles cleanly — all the transitional breakage from Tasks 1-3 is now resolved.

- [ ] **Step 7: Run the core service's unit and integration tests**

Run: `task test:core:unit`
Expected: all PASS.

Run: `task test:core:integration`
Expected: all PASS (requires Docker).

- [ ] **Step 8: Commit**

```bash
gofmt -w services/core/internal/infra/inbox/email_received_consumer.go services/core/internal/infra/inbox/email_received_consumer_test.go services/core/cmd/email-received-consumer/main.go
git add services/core/internal/infra/inbox services/core/cmd/email-received-consumer
git commit -m "Rewire email-received consumer onto libs/inbox.Consumer[T]"
```

---

## Task 5: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run every unit test suite**

```bash
task test:core:unit
task test:email:unit
task test:libs:unit
```
Expected: all PASS.

- [ ] **Step 2: Run every integration test suite (requires Docker)**

```bash
task test:core:integration
task test:email:integration
go test ./libs/outbox/... ./libs/inbox/... ./libs/schemaregistry/... -v
```
Expected: all PASS.

- [ ] **Step 3: Full build, vet, format, lint**

```bash
go build ./...
go vet ./...
gofmt -l $(find . -name "*.go" -not -path "./vendor/*")
task lint:all
```
Expected: `go build`/`go vet` exit 0; `gofmt -l` prints nothing new (the pre-existing, unrelated `services/email/internal/infra/rest/api/errors.go` formatting issue may still show — that's not part of this change, leave it); `task lint:all` reports 0 new issues.

- [ ] **Step 4: Confirm no stale references to the renamed/removed names**

```bash
grep -rn "outbox\.NewProducer\b\|outbox\.Producer\b" --include="*.go" . | grep -v "_test.go\|RawProducer\|libsoutbox.Producer\[" 
grep -rn "inbox\.NewConsumer\b\|inbox\.Consumer\b" --include="*.go" . | grep -v "_test.go\|KafkaConsumer\|RawConsumer\|libsinbox.Consumer\["
```
Expected: no output from either (all references now use the new names, or refer to the new generic `Producer[T]`/`Consumer[T]` types, which is expected and fine).

- [ ] **Step 5: End-to-end manual smoke test**

Rebuild and restart both sidecars against the live dev environment, then confirm a real message flows through the full pipeline with the new code:

```bash
cd dev && docker compose up -d --build --no-deps email-received-producer email-received-consumer
sleep 5
docker inspect email-received-producer email-received-consumer --format '{{.Name}}: RestartCount={{.RestartCount}} Status={{.State.Status}}'
```
Expected: both `RestartCount=0`, `Status=running`.

Then insert a real outbox row directly (bypassing the mail-fetch step, out of scope here) and confirm it flows through:

```bash
docker exec postgres psql -U planeo -d mail -c "
INSERT INTO mails (message_id, setting_id, organization_id, subject, sender, body, date)
VALUES ('plan-verify-1', 1, 1, 'Plan verification', 'verify@example.com', 'Verifying the generic engine', now())
RETURNING id;
"
docker exec postgres psql -U planeo -d mail -c "
INSERT INTO outbox (mail_id, topic, key, payload)
VALUES (
  (SELECT id FROM mails WHERE message_id = 'plan-verify-1'),
  'email-received', '1',
  '{\"subject\":\"Plan verification\",\"body\":\"Verifying the generic engine\",\"from\":\"verify@example.com\",\"date\":\"2026-01-01T00:00:00Z\",\"messageId\":\"plan-verify-1\",\"organizationId\":1}'::jsonb
) RETURNING id, status;
"
sleep 4
docker exec postgres psql -U planeo -d planeo -c "SELECT id, subject, reference_id FROM requests WHERE reference_id='plan-verify-1';"
```
Expected: the `requests` row exists with `subject = 'Plan verification'`, proving the full path (insert → `Producer[T]` polls, JSON-decodes, Avro-encodes, sends → Kafka → `Consumer[T]` ingests, polls, Avro-decodes, maps, calls `ProcessEmailReceived` → a `request` row is created) works end to end with the new generic engine.

Clean up the test data:

```bash
docker exec postgres psql -U planeo -d planeo -c "DELETE FROM requests WHERE reference_id='plan-verify-1';"
docker exec postgres psql -U planeo -d mail -c "DELETE FROM outbox WHERE mail_id = (SELECT id FROM mails WHERE message_id = 'plan-verify-1');"
docker exec postgres psql -U planeo -d mail -c "DELETE FROM mails WHERE message_id = 'plan-verify-1';"
```

- [ ] **Step 6: Commit any final fixups discovered during verification**

If any of the above steps required a fix, commit it with a message describing what verification step caught it.
