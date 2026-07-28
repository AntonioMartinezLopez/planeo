package inbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"planeo/libs/eventschemas"
	libsinbox "planeo/libs/inbox"
	"planeo/libs/schemaregistry"
	domaininbox "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/domain/inbox/mocks"
	"planeo/services/core/internal/infra/inbox"

	"github.com/stretchr/testify/mock"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// fakeKafkaConsumer is a hand-rolled KafkaConsumer double — a single-method
// startup interface, not worth mockery generation, matching this
// codebase's existing convention for narrow infra-owned interfaces.
type fakeKafkaConsumer struct {
	err   error
	calls int
}

func (f *fakeKafkaConsumer) Run(ctx context.Context) error {
	f.calls++
	return f.err
}

// fakeDecoder stands in for a real Avro decoder in tests that only care
// about the decode-failure path — TestPollOnce_decodesAndProcesses below is
// the one test that needs a real, registry-backed decoder.
type fakeDecoder struct {
	err error
}

func (f *fakeDecoder) Decode(ctx context.Context, b []byte) (eventschemas.EmailReceived, error) {
	return eventschemas.EmailReceived{}, f.err
}

func testConfig() inbox.Config {
	return inbox.Config{
		Topic: "email-received", InstanceID: "instance-a",
		BatchSize: 10, MaxAttempts: 5, ClaimTTL: 30 * time.Second,
	}
}

func TestEmailReceivedConsumer(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("propagates a FetchBatch error without processing any record", func(t *testing.T) {
		mockService := mocks.NewMockService(t)
		fetchErr := errors.New("fetch failed")
		mockService.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return(nil, fetchErr)

		consumer := inbox.NewEmailReceivedConsumer(
			inbox.Deps{KafkaConsumer: &fakeKafkaConsumer{}, Service: mockService, Decoder: &fakeDecoder{}},
			testConfig(),
		)

		err := consumer.PollOnce(context.Background())

		if !errors.Is(err, fetchErr) {
			t.Errorf("got err %v, want %v", err, fetchErr)
		}
	})

	t.Run("marks a record failed and continues when decoding fails", func(t *testing.T) {
		mockService := mocks.NewMockService(t)
		mockService.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return([]libsinbox.Record{{ID: 1, Topic: "email-received", Payload: []byte("not-a-valid-avro-envelope")}}, nil)
		mockService.EXPECT().
			MarkFailed(context.Background(), int64(1), mock.Anything, 5).
			Return(nil)

		consumer := inbox.NewEmailReceivedConsumer(
			inbox.Deps{KafkaConsumer: &fakeKafkaConsumer{}, Service: mockService, Decoder: &fakeDecoder{err: errors.New("boom")}},
			testConfig(),
		)

		if err := consumer.PollOnce(context.Background()); err != nil {
			t.Fatalf("PollOnce should not abort the batch on a decode failure, got: %v", err)
		}
	})

	t.Run("Run starts the raw Kafka consumer and propagates its startup error", func(t *testing.T) {
		mockService := mocks.NewMockService(t)
		kafkaErr := errors.New("broker unreachable")
		fakeConsumer := &fakeKafkaConsumer{err: kafkaErr}

		consumer := inbox.NewEmailReceivedConsumer(
			inbox.Deps{KafkaConsumer: fakeConsumer, Service: mockService, Decoder: &fakeDecoder{}},
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

func startRegistry(t *testing.T) *schemaregistry.Serde {
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
	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: "http://" + host + ":" + port.Port() + "/apis/ccompat/v7"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// RegisterDecode only looks a subject up (see its doc comment and
	// TestRegisterDecode_failsFastOnUnknownSubject in libs/schemaregistry)
	// — it never creates one, and it only wires a DecodeFn, not an
	// EncodeFn. This test also needs to Encode(serde, wire) itself to
	// produce genuinely-encoded bytes, so it uses RegisterEncode instead:
	// it creates the subject AND wires both an EncodeFn and DecodeFn on
	// serde, giving one serde that can do both halves of this test. In
	// production, the subject is created ahead of time by
	// cmd/registerschemas and the sidecar's own decoder is decode-only
	// (see cmd/main.go) — this divergence is purely a test-fixture
	// convenience for driving both Encode and Decode from a single process.
	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[eventschemas.EmailReceived](ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailReceivedSchema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}
	return serde
}

// TestPollOnce_decodesAndProcesses proves the event handed to
// ProcessEmailReceived is genuinely Avro-decoded (via a real registry), not
// just passed through — the one test in this file that needs a live
// Apicurio container.
func TestPollOnce_decodesAndProcesses(t *testing.T) {
	serde := startRegistry(t)
	decoder := schemaregistry.NewDecoder[eventschemas.EmailReceived](serde, eventschemas.EmailReceivedSchema)

	wire := eventschemas.EmailReceived{
		Subject: "hi", Body: "body", From: "a@b.com",
		Date:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MessageID: "msg-1", OrganizationID: 7,
	}
	encoded, err := schemaregistry.Encode(serde, wire)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	mockService := mocks.NewMockService(t)
	mockService.EXPECT().
		FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
		Return([]libsinbox.Record{{ID: 1, Topic: "email-received", Payload: encoded}}, nil)
	mockService.EXPECT().
		ProcessEmailReceived(context.Background(), int64(1), domaininbox.EmailReceivedEvent{
			Subject: "hi", Body: "body", From: "a@b.com",
			Date:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			MessageID: "msg-1", OrganizationID: 7,
		}, 5).
		Return(nil)

	consumer := inbox.NewEmailReceivedConsumer(
		inbox.Deps{KafkaConsumer: &fakeKafkaConsumer{}, Service: mockService, Decoder: decoder},
		testConfig(),
	)

	if err := consumer.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
}
