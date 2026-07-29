package outbox

import (
	"context"
	"time"

	"planeo/libs/eventschemas"
	libsoutbox "planeo/libs/outbox"
	domainoutbox "planeo/services/email/internal/domain/outbox"
)

// EmailReceivedProducer is libs/outbox's generic engine, fixed to the
// email-received topic/subject/schema. This file's only job is that
// fixing: no loop, no encode logic, no Kafka/registry construction of its
// own — all of that lives in libs/outbox.Producer[T].
type EmailReceivedProducer = libsoutbox.Producer[eventschemas.EmailReceived]

// Config is flat all the way through — no kafka.Config, no
// schemaregistry.Config — so this package, and cmd/main.go one level up,
// need neither import just to build one.
type Config struct {
	KafkaBrokers           []string
	KafkaUsername          string
	KafkaPassword          string
	KafkaMechanism         string
	KafkaTLSEnabled        bool
	RegistryURL            string
	InstanceID             string
	BatchSize, MaxAttempts int
	ClaimTTL, PollInterval time.Duration
}

func NewEmailReceivedProducer(ctx context.Context, service domainoutbox.Service, cfg Config) (*EmailReceivedProducer, error) {
	return libsoutbox.NewFromConfig[eventschemas.EmailReceived](ctx,
		libsoutbox.BuildConfig{
			KafkaBrokers:    cfg.KafkaBrokers,
			KafkaUsername:   cfg.KafkaUsername,
			KafkaPassword:   cfg.KafkaPassword,
			KafkaMechanism:  cfg.KafkaMechanism,
			KafkaTLSEnabled: cfg.KafkaTLSEnabled,
			RegistryURL:     cfg.RegistryURL,
			Subject:         eventschemas.EmailReceivedSubject,
			Schema:          eventschemas.EmailReceivedSchema,
		},
		libsoutbox.Deps[eventschemas.EmailReceived]{
			FetchBatch:    service.FetchBatch,
			MarkProcessed: service.MarkProcessed,
			MarkFailed:    service.MarkFailed,
		},
		libsoutbox.Config{
			Topic: eventschemas.EmailReceivedTopic, InstanceID: cfg.InstanceID,
			BatchSize: cfg.BatchSize, MaxAttempts: cfg.MaxAttempts,
			ClaimTTL: cfg.ClaimTTL, PollInterval: cfg.PollInterval,
		},
	)
}
