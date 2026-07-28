package schemaregistry_test

import (
	"context"
	"testing"
	"time"

	"planeo/libs/schemaregistry"

	avro "github.com/iskorotkov/avro/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/twmb/franz-go/pkg/sr"
)

type testRecordV1 struct {
	Name string `avro:"name" json:"name"`
}

type testRecordV2 struct {
	Name     string `avro:"name" json:"name"`
	Priority string `avro:"priority" json:"priority"`
}

const testSchemaV1 = `{
  "type": "record", "name": "TestRecord", "namespace": "planeo.test",
  "fields": [{"name": "name", "type": "string"}]
}`

const testSchemaV2 = `{
  "type": "record", "name": "TestRecord", "namespace": "planeo.test",
  "fields": [
    {"name": "name", "type": "string"},
    {"name": "priority", "type": ["null", "string"], "default": null}
  ]
}`

func startRegistry(t *testing.T) *sr.Client {
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
	return client
}

func TestRegisterEncodeAndDecode_roundTrip(t *testing.T) {
	client := startRegistry(t)
	schema := avro.MustParse(testSchemaV1)

	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[testRecordV1](context.Background(), serde, "test-record-value", schema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}

	encoded, err := schemaregistry.Encode(serde, testRecordV1{Name: "hello"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var out testRecordV1
	if err := schemaregistry.Decode(context.Background(), serde, encoded, schema, &out); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out.Name != "hello" {
		t.Errorf("got Name=%q, want %q", out.Name, "hello")
	}
}

// TestDecode_schemaEvolution proves the discover-on-ErrNotRegistered path:
// a FRESH Serde (simulating a consumer process that has never seen the
// producer's schema ID) must still correctly decode a message written
// with an older schema into a newer reader schema with an additional
// defaulted field, without ever having called RegisterEncode/RegisterDecode
// for that specific ID.
func TestDecode_schemaEvolution(t *testing.T) {
	client := startRegistry(t)
	v1Schema := avro.MustParse(testSchemaV1)
	v2Schema := avro.MustParse(testSchemaV2)

	producerSerde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[testRecordV1](context.Background(), producerSerde, "test-record-evolution-value", v1Schema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}
	encoded, err := schemaregistry.Encode(producerSerde, testRecordV1{Name: "hello"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	consumerSerde := schemaregistry.NewSerde(client) // fresh - has never registered this ID
	var out testRecordV2
	if err := schemaregistry.Decode(context.Background(), consumerSerde, encoded, v2Schema, &out); err != nil {
		t.Fatalf("Decode with v2 reader schema: %v", err)
	}
	if out.Name != "hello" {
		t.Errorf("got Name=%q, want %q", out.Name, "hello")
	}
	if out.Priority != "" {
		t.Errorf("got Priority=%q, want empty (zero value for a null-defaulted field the writer never had)", out.Priority)
	}
}

func TestRegisterDecode_failsFastOnUnknownSubject(t *testing.T) {
	client := startRegistry(t)
	schema := avro.MustParse(testSchemaV1)
	serde := schemaregistry.NewSerde(client)

	// A subject that was never registered by anyone (no prior RegisterEncode
	// call for it) — LookupSchema has nothing to find, so this must error
	// rather than silently succeed. This is what RegisterDecode's use as a
	// startup fail-fast check (design spec Section 5) depends on.
	if err := schemaregistry.RegisterDecode[testRecordV1](context.Background(), serde, "never-registered-value", schema); err == nil {
		t.Fatal("expected an error for a subject with no registered schema, got nil")
	}
}
