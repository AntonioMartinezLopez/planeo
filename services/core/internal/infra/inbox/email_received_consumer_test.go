package inbox_test

import (
	"context"
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
	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "8080")
	client, err := schemaregistry.NewClient(schemaregistry.Config{URL: "http://" + host + ":" + port.Port() + "/apis/ccompat/v7"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// The brief's original helper called RegisterDecode directly against a
	// brand-new registry, but RegisterDecode only looks a subject up (see
	// its doc comment and TestRegisterDecode_failsFastOnUnknownSubject in
	// libs/schemaregistry) — it never creates one, and it only wires a
	// DecodeFn, not an EncodeFn. This test also needs to Encode(serde, wire)
	// itself to produce genuinely-encoded bytes, so it uses RegisterEncode
	// instead: it creates the subject (Register, internally) AND wires both
	// an EncodeFn and DecodeFn on serde, giving one serde that can do both
	// halves of this test. In production, the subject is created ahead of
	// time by cmd/registerschemas and the sidecar's own serde is
	// decode-only (see cmd/main.go, Step 6) — this divergence is purely a
	// test-fixture convenience for driving both Encode and Decode from a
	// single process.
	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[eventschemas.EmailReceived](ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailReceivedSchema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}
	return serde
}

func TestPollOnce_decodesAndProcesses(t *testing.T) {
	serde := startRegistry(t)

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

	consumer := inbox.NewEmailReceivedConsumer(mockService, serde, "email-received", "instance-a", 10, 5, 30*time.Second)

	if err := consumer.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce: %v", err)
	}
}

func TestPollOnce_decodeFailureMarksFailedAndContinues(t *testing.T) {
	serde := startRegistry(t)

	mockService := mocks.NewMockService(t)
	mockService.EXPECT().
		FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
		Return([]libsinbox.Record{
			{ID: 1, Topic: "email-received", Payload: []byte("not-a-valid-avro-envelope")},
		}, nil)
	mockService.EXPECT().
		MarkFailed(context.Background(), int64(1), mock.Anything, 5).
		Return(nil)

	consumer := inbox.NewEmailReceivedConsumer(mockService, serde, "email-received", "instance-a", 10, 5, 30*time.Second)

	if err := consumer.PollOnce(context.Background()); err != nil {
		t.Fatalf("PollOnce should not abort the batch on a decode failure, got: %v", err)
	}
}
