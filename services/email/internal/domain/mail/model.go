package mail

import "time"

type Mail struct {
	ID             int       `db:"id"`
	MessageID      string    `db:"message_id"`
	SettingID      int       `db:"setting_id"`
	OrganizationID int       `db:"organization_id"`
	Subject        string    `db:"subject"`
	Sender         string    `db:"sender"`
	Body           string    `db:"body"`
	Date           time.Time `db:"date"`
	CreatedAt      time.Time `db:"created_at"`
}

type NewMail struct {
	MessageID      string
	SettingID      int
	OrganizationID int
	Subject        string
	Sender         string
	Body           string
	Date           time.Time
}

// EmailReceivedFields is the plain domain shape of an email-received event —
// no wire-format knowledge (no avro/json tags, no schema import). The
// mapping into the wire type lives in the infra repository (see
// infra/postgres/outbox_repository.go), not here.
type EmailReceivedFields struct {
	Subject        string
	Body           string
	From           string
	Date           time.Time
	MessageID      string
	OrganizationID int
}

// OutboxEvent is the Kafka event to be durably queued alongside a NewMail,
// in the same local transaction. Topic is not carried here — the infra
// repository sets it from eventschemas.EmailReceivedTopic, since that
// constant lives in a wire-format package this domain layer must not import.
type OutboxEvent struct {
	Key    []byte
	Fields EmailReceivedFields
}

// RawFetchedMail is the raw data a fetch-side adapter (e.g. IMAP) hands to
// this domain — it carries no knowledge of Kafka, topics, or event
// payloads; SaveFetchedMails builds those internally.
type RawFetchedMail struct {
	MessageID      string
	SettingID      int
	OrganizationID int
	Subject        string
	Sender         string
	Body           string
	Date           time.Time
	UID            uint32
}

// SaveResult reports, per fetched mail, its IMAP UID (so the caller can
// mark it seen) and whether this call actually inserted a new mails row
// (false means it was already recorded from a prior attempt — still safe
// to mark seen, but no new outbox event was created for it this time).
type SaveResult struct {
	UID      uint32
	Inserted bool
}
