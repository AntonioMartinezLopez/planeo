// Package contracts defines the wire-format contracts for domain events
// that cross service boundaries. These types are intentionally kept in a
// separate package with zero dependencies beyond the standard library, so
// that producers can depend on contracts without pulling in any message
// queue client libraries.
package contracts

import "time"

// EmailCreatedPayload is the wire-format contract for the "email received"
// domain event: the shape a producer (services/email's outbox) serializes
// into an outbox row's payload, and a consumer (services/core's Kafka
// subscribe handler) deserializes back out. Kept in its own package, with
// no dependencies beyond the standard library, so a producer can depend on
// it without pulling in any Kafka client code.
type EmailCreatedPayload struct {
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	From           string    `json:"from"`
	Date           time.Time `json:"date"`
	MessageID      string    `json:"messageId"`
	OrganizationId int       `json:"organizationId"`
}

// EmailReceivedTopic is the Kafka topic both the outbox producer and the
// libs/events subscriber use for EmailCreatedPayload events.
const EmailReceivedTopic = "email-received"
