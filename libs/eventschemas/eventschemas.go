package eventschemas

import (
	_ "embed"

	avro "github.com/iskorotkov/avro/v2"
)

const (
	EmailReceivedTopic   = "email-received"
	EmailReceivedSubject = "email-received-value"
)

//go:embed schemas/email_received.avsc
var emailCreatedSchemaRaw string

// EmailReceivedSchema is the parsed form of schemas/email_received.avsc — the
// single source of truth this type and its registered version are both
// derived from.
var EmailReceivedSchema = avro.MustParse(emailCreatedSchemaRaw)

// Schemas returns every schema this package owns, keyed by registry subject
// name. cmd/registerschemas (Task 4) walks this generically — it never
// hardcodes which event types exist.
func Schemas() map[string]avro.Schema {
	return map[string]avro.Schema{
		EmailReceivedSubject: EmailReceivedSchema,
	}
}
