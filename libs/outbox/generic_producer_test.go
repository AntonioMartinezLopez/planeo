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
			KafkaBrokers:   []string{"localhost:0"},
			KafkaMechanism: "SCRAM-SHA-256",
			RegistryURL:    registryURL,
			Subject:        "test-wire-value",
			Schema:         testWireSchema,
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
