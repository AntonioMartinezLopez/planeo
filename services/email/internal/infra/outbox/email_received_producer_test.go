package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"planeo/libs/eventschemas"
	libsoutbox "planeo/libs/outbox"
	"planeo/libs/schemaregistry"
	"planeo/services/email/internal/domain/outbox/mocks"
	"planeo/services/email/internal/infra/outbox"

	"github.com/stretchr/testify/mock"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// fakeProducer is a hand-rolled Producer double — a raw Kafka sender is a
// single-method interface, not worth mockery generation, matching this
// codebase's existing convention for narrow infra-owned interfaces.
// failOnCall, if non-zero, makes only the Nth call (1-indexed) fail — lets a
// single fake exercise "one record in a batch fails, the rest succeed".
type fakeProducer struct {
	gotTopic       string
	gotKey, gotVal []byte
	err            error
	failOnCall     int
	calls          int
}

func (f *fakeProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	f.calls++
	f.gotTopic, f.gotKey, f.gotVal = topic, key, value
	if f.failOnCall == 0 || f.calls == f.failOnCall {
		return f.err
	}
	return nil
}

// fakeEncoder stands in for a real Avro encoder in tests that never reach
// the encode step (or don't care what bytes result) — TestPollOnce_encodesAndSends
// below is the one test that needs a real, registry-backed encoder.
type fakeEncoder struct {
	err error
}

func (f *fakeEncoder) Encode(v eventschemas.EmailReceived) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte("encoded"), nil
}

func testConfig() outbox.Config {
	return outbox.Config{
		Topic: "email-received", InstanceID: "instance-a",
		BatchSize: 10, MaxAttempts: 5, ClaimTTL: 30 * time.Second,
	}
}

func TestEmailReceivedProducer(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("propagates a FetchBatch error without processing any record", func(t *testing.T) {
		mockService := mocks.NewMockService(t)
		fetchErr := errors.New("fetch failed")
		mockService.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return(nil, fetchErr)

		producer := outbox.NewEmailReceivedProducer(
			outbox.Deps{Service: mockService, Producer: &fakeProducer{}, Encoder: &fakeEncoder{}},
			testConfig(),
		)

		err := producer.PollOnce(context.Background())

		if !errors.Is(err, fetchErr) {
			t.Errorf("got err %v, want %v", err, fetchErr)
		}
	})

	t.Run("processes every fetched record, even if one fails to send", func(t *testing.T) {
		mockService := mocks.NewMockService(t)

		okPayload, err := json.Marshal(eventschemas.EmailReceived{Subject: "ok"})
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		records := []libsoutbox.Record{
			{ID: 1, Topic: "email-received", Payload: okPayload},
			{ID: 2, Topic: "email-received", Payload: okPayload},
		}
		mockService.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return(records, nil)
		mockService.EXPECT().MarkFailed(context.Background(), int64(1), mock.Anything, 5).Return(nil)
		mockService.EXPECT().MarkProcessed(context.Background(), int64(2)).Return(nil)

		producer := outbox.NewEmailReceivedProducer(
			outbox.Deps{Service: mockService, Producer: &fakeProducer{err: errors.New("boom"), failOnCall: 1}, Encoder: &fakeEncoder{}},
			testConfig(),
		)

		if err := producer.PollOnce(context.Background()); err != nil {
			t.Fatalf("PollOnce: %v", err)
		}
	})

	t.Run("marks a record failed and continues when its payload is not valid JSON", func(t *testing.T) {
		mockService := mocks.NewMockService(t)

		badRecord := libsoutbox.Record{ID: 1, Topic: "email-received", Payload: []byte("not-json")}
		mockService.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return([]libsoutbox.Record{badRecord}, nil)
		mockService.EXPECT().MarkFailed(context.Background(), int64(1), mock.Anything, 5).Return(nil)

		producer := outbox.NewEmailReceivedProducer(
			outbox.Deps{Service: mockService, Producer: &fakeProducer{}, Encoder: &fakeEncoder{}},
			testConfig(),
		)

		if err := producer.PollOnce(context.Background()); err != nil {
			t.Fatalf("PollOnce should not abort the batch on a bad payload, got: %v", err)
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
	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[eventschemas.EmailReceived](ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailReceivedSchema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}
	return serde
}

// TestPollOnce_encodesAndSends proves the bytes handed to Producer are
// genuinely Avro-encoded (via a real registry), not just passed through —
// the one test in this file that needs a live Apicurio container.
func TestPollOnce_encodesAndSends(t *testing.T) {
	serde := startRegistry(t)
	encoder := schemaregistry.NewEncoder[eventschemas.EmailReceived](serde)
	fake := &fakeProducer{}
	mockService := mocks.NewMockService(t)

	wire := eventschemas.EmailReceived{
		Subject: "hi", Body: "body", From: "a@b.com",
		MessageID: "msg-1", OrganizationID: 1,
	}
	jsonPayload, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	record := libsoutbox.Record{ID: 1, Topic: "email-received", Key: []byte("1"), Payload: jsonPayload}

	mockService.EXPECT().
		FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
		Return([]libsoutbox.Record{record}, nil)
	mockService.EXPECT().MarkProcessed(context.Background(), int64(1)).Return(nil)

	producer := outbox.NewEmailReceivedProducer(
		outbox.Deps{Service: mockService, Producer: fake, Encoder: encoder},
		testConfig(),
	)

	if err := producer.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}

	if fake.gotTopic != "email-received" {
		t.Errorf("got topic %q, want %q", fake.gotTopic, "email-received")
	}
	if string(fake.gotKey) != "1" {
		t.Errorf("got key %q, want %q", fake.gotKey, "1")
	}

	// Decode what was actually sent using the same serde, proving the bytes
	// are genuinely Avro-encoded, not just passed through unchanged.
	var decoded eventschemas.EmailReceived
	if err := schemaregistry.Decode(context.Background(), serde, fake.gotVal, eventschemas.EmailReceivedSchema, &decoded); err != nil {
		t.Fatalf("decoding what was sent: %v", err)
	}
	if decoded.Subject != "hi" || decoded.OrganizationID != 1 {
		t.Errorf("got %+v, want Subject=hi OrganizationID=1", decoded)
	}
}
