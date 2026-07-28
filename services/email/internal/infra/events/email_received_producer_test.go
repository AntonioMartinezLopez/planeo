package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"planeo/libs/eventschemas"
	"planeo/libs/schemaregistry"
	"planeo/services/email/internal/infra/events"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type fakeProducer struct {
	gotTopic       string
	gotKey, gotVal []byte
	err            error
}

func (f *fakeProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	f.gotTopic, f.gotKey, f.gotVal = topic, key, value
	return f.err
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
	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "8080")
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

func TestEmailReceivedProducer_encodesAndForwards(t *testing.T) {
	serde := startRegistry(t)
	fake := &fakeProducer{}
	producer := events.NewEmailReceivedProducer(fake, serde)

	wire := eventschemas.EmailReceived{
		Subject: "hi", Body: "body", From: "a@b.com",
		MessageID: "msg-1", OrganizationID: 1,
	}
	jsonPayload, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	err = producer.ProduceSync(context.Background(), "email-received", []byte("1"), jsonPayload)
	if err != nil {
		t.Fatalf("ProduceSync: %v", err)
	}

	if fake.gotTopic != "email-received" {
		t.Errorf("got topic %q, want %q", fake.gotTopic, "email-received")
	}
	if string(fake.gotKey) != "1" {
		t.Errorf("got key %q, want %q", fake.gotKey, "1")
	}

	// Decode what was actually passed to the inner producer using the same
	// serde, proving the bytes are genuinely Avro-encoded, not just passed
	// through unchanged.
	var decoded eventschemas.EmailReceived
	if err := schemaregistry.Decode(context.Background(), serde, fake.gotVal, eventschemas.EmailReceivedSchema, &decoded); err != nil {
		t.Fatalf("decoding what was passed to inner producer: %v", err)
	}
	if decoded.Subject != "hi" || decoded.OrganizationID != 1 {
		t.Errorf("got %+v, want Subject=hi OrganizationID=1", decoded)
	}
}

func TestEmailReceivedProducer_badJSONPayload(t *testing.T) {
	serde := startRegistry(t)
	fake := &fakeProducer{}
	producer := events.NewEmailReceivedProducer(fake, serde)

	err := producer.ProduceSync(context.Background(), "email-received", []byte("1"), []byte("not-json"))
	if err == nil {
		t.Fatal("expected an error for malformed JSON payload, got nil")
	}
	var target *json.SyntaxError
	if !errors.As(err, &target) {
		t.Errorf("expected error to wrap a json.SyntaxError, got %v", err)
	}
}
