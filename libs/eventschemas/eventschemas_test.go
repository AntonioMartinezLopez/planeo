package eventschemas_test

import (
	"testing"

	"planeo/libs/eventschemas"

	avro "github.com/iskorotkov/avro/v2"
)

func TestSchemas_includesEmailReceived(t *testing.T) {
	schemas := eventschemas.Schemas()
	schema, ok := schemas[eventschemas.EmailReceivedSubject]
	if !ok {
		t.Fatalf("Schemas() missing subject %q", eventschemas.EmailReceivedSubject)
	}
	if schema.String() != eventschemas.EmailReceivedSchema.String() {
		t.Errorf("Schemas()[%q] does not match EmailReceivedSchema", eventschemas.EmailReceivedSubject)
	}
}

func TestEmailReceived_avroRoundTrip(t *testing.T) {
	in := eventschemas.EmailReceived{
		Subject:        "hello",
		Body:           "world",
		From:           "a@b.com",
		MessageID:      "msg-1",
		OrganizationID: 7,
	}
	data, err := avro.Marshal(eventschemas.EmailReceivedSchema, in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out eventschemas.EmailReceived
	if err := avro.Unmarshal(eventschemas.EmailReceivedSchema, data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Subject != in.Subject || out.Body != in.Body || out.From != in.From ||
		out.MessageID != in.MessageID || out.OrganizationID != in.OrganizationID {
		t.Errorf("got %+v, want %+v", out, in)
	}
}
