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
			KafkaBrokers:   []string{"localhost:0"},
			KafkaMechanism: "SCRAM-SHA-256",
			GroupName:      "test-group",
			RegistryURL:    registryURL,
			Subject:        "test-wire-value",
			Schema:         testWireSchema,
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
