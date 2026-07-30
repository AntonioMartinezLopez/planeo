package schemaregistry_test

import (
	"context"
	"testing"

	"planeo/libs/schemaregistry"

	avro "github.com/iskorotkov/avro/v2"
)

func TestEncoderDecoder_roundTrip(t *testing.T) {
	client := startRegistry(t)
	schema := avro.MustParse(testSchemaV1)

	serde := schemaregistry.NewSerde(client)
	if err := schemaregistry.RegisterEncode[testRecordV1](context.Background(), serde, "codec-test-record-value", schema); err != nil {
		t.Fatalf("RegisterEncode: %v", err)
	}

	encoder := schemaregistry.NewEncoder[testRecordV1](serde)
	encoded, err := encoder.Encode(testRecordV1{Name: "hello"})
	if err != nil {
		t.Fatalf("Encoder.Encode: %v", err)
	}

	decoder := schemaregistry.NewDecoder[testRecordV1](serde, schema)
	out, err := decoder.Decode(context.Background(), encoded)
	if err != nil {
		t.Fatalf("Decoder.Decode: %v", err)
	}
	if out.Name != "hello" {
		t.Errorf("got Name=%q, want %q", out.Name, "hello")
	}
}
