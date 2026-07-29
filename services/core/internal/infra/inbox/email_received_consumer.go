package inbox

import (
	"context"
	"time"

	"planeo/libs/eventschemas"
	libsinbox "planeo/libs/inbox"
	domaininbox "planeo/services/core/internal/domain/inbox"
)

// EmailReceivedConsumer is libs/inbox's generic engine, fixed to the
// email-received topic/subject/schema. This file's only two jobs are that
// fixing, and mapping the Avro-decoded wire type into the domain-owned
// event type ProcessEmailReceived expects — no loop, no decode logic, no
// Kafka/registry construction of its own.
type EmailReceivedConsumer = libsinbox.Consumer[eventschemas.EmailReceived]

type Config struct {
	KafkaBrokers           []string
	KafkaUsername          string
	KafkaPassword          string
	KafkaMechanism         string
	KafkaTLSEnabled        bool
	GroupName              string
	RegistryURL            string
	InstanceID             string
	BatchSize, MaxAttempts int
	ClaimTTL, PollInterval time.Duration
}

func NewEmailReceivedConsumer(ctx context.Context, service domaininbox.Service, cfg Config) (*EmailReceivedConsumer, error) {
	return libsinbox.NewFromConfig[eventschemas.EmailReceived](ctx,
		libsinbox.BuildConfig{
			KafkaBrokers:    cfg.KafkaBrokers,
			KafkaUsername:   cfg.KafkaUsername,
			KafkaPassword:   cfg.KafkaPassword,
			KafkaMechanism:  cfg.KafkaMechanism,
			KafkaTLSEnabled: cfg.KafkaTLSEnabled,
			GroupName:       cfg.GroupName,
			RegistryURL:     cfg.RegistryURL,
			Subject:         eventschemas.EmailReceivedSubject,
			Schema:          eventschemas.EmailReceivedSchema,
		},
		service.Save,
		libsinbox.Deps[eventschemas.EmailReceived]{
			FetchBatch: service.FetchBatch,
			Process: func(ctx context.Context, id int64, wire eventschemas.EmailReceived, maxAttempts int) error {
				return service.ProcessEmailReceived(ctx, id, MapEmailReceivedEvent(wire), maxAttempts)
			},
			MarkFailed: service.MarkFailed,
		},
		libsinbox.Config{
			Topic: eventschemas.EmailReceivedTopic, InstanceID: cfg.InstanceID,
			BatchSize: cfg.BatchSize, MaxAttempts: cfg.MaxAttempts,
			ClaimTTL: cfg.ClaimTTL, PollInterval: cfg.PollInterval,
		},
	)
}

// MapEmailReceivedEvent converts the Avro-decoded wire type into the
// domain-owned event type ProcessEmailReceived expects — not
// eventschemas.EmailReceived, the wire type Consumer[T] decodes into, by
// the original Kafka spec's own deliberate design (keep domain/inbox free
// of wire-format types). This six-field mapping is the one piece of real
// logic this adapter carries. Exported (rather than left as an inline
// closure, or unexported) so it's directly testable without constructing a
// full Consumer[T] — which would otherwise require a live schema registry
// just to exercise a struct copy.
func MapEmailReceivedEvent(wire eventschemas.EmailReceived) domaininbox.EmailReceivedEvent {
	return domaininbox.EmailReceivedEvent{
		Subject:        wire.Subject,
		Body:           wire.Body,
		From:           wire.From,
		Date:           wire.Date,
		MessageID:      wire.MessageID,
		OrganizationID: wire.OrganizationID,
	}
}
