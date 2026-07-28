package inbox

import "time"

// ExtractorOutput holds the fields the LLM extracted from a request.
type ExtractorOutput struct {
	Address string
	Name    string
	Phone   string
}

// RequestData is the LLM classifier's input shape for a single request.
type RequestData struct {
	Subject string
	Text    string
}

// EmailReceivedEvent is the already-decoded shape of an email.received
// event. Decoding (Avro/wire-format concerns) is the driving adapter's
// responsibility, not the domain's.
type EmailReceivedEvent struct {
	Subject        string
	Body           string
	From           string
	Date           time.Time
	MessageID      string
	OrganizationID int
}
