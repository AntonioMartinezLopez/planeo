package inbox

import (
	"context"
	"time"

	"planeo/libs/eventschemas"
	"planeo/libs/logger"
	domaininbox "planeo/services/core/internal/domain/inbox"
)

// KafkaConsumer starts the raw Kafka-to-inbox-table ingestion loop this
// consumer relies on. Declared here (rather than depending on
// *libsinbox.Consumer directly) so it can be faked in unit tests without a
// live broker; *libsinbox.Consumer satisfies this structurally, no wrapper
// needed.
type KafkaConsumer interface {
	Run(ctx context.Context) error
}

// Decoder Avro-decodes a raw inbox payload into an EmailReceived value.
// Declared here (rather than depending on *schemaregistry.Serde directly)
// so it can be mocked in unit tests without a live registry;
// *schemaregistry.Decoder[eventschemas.EmailReceived] satisfies this
// structurally, no wrapper needed.
type Decoder interface {
	Decode(ctx context.Context, b []byte) (eventschemas.EmailReceived, error)
}

// Deps are EmailReceivedConsumer's collaborators: the raw Kafka ingestion
// loop, the domain service (fetch pending rows, process or record each
// outcome), and the Avro decoder. Grouped separately from Config so call
// sites read as "these are the things it talks to" vs "these are the knobs
// it runs with".
type Deps struct {
	KafkaConsumer KafkaConsumer
	Service       domaininbox.Service
	Decoder       Decoder
}

// Config is EmailReceivedConsumer's scalar, topic/instance-specific
// configuration.
type Config struct {
	Topic        string
	InstanceID   string
	BatchSize    int
	MaxAttempts  int
	ClaimTTL     time.Duration
	PollInterval time.Duration
}

// EmailReceivedConsumer owns everything the "email-received" inbox needs on
// the consume side: the raw Kafka-to-DB ingestion loop, polling the domain
// service for a batch of claimed rows, Avro-decoding each payload, and
// handing the decoded event to the domain service — one type, no separate
// raw-consumer/driving-adapter split. serde is already registered for
// EmailReceived by the time it reaches here (see cmd/main.go).
type EmailReceivedConsumer struct {
	kafkaConsumer KafkaConsumer
	service       domaininbox.Service
	decoder       Decoder
	topic         string
	instanceID    string
	batchSize     int
	maxAttempts   int
	claimTTL      time.Duration
	pollInterval  time.Duration
}

func NewEmailReceivedConsumer(deps Deps, cfg Config) *EmailReceivedConsumer {
	return &EmailReceivedConsumer{
		kafkaConsumer: deps.KafkaConsumer,
		service:       deps.Service,
		decoder:       deps.Decoder,
		topic:         cfg.Topic,
		instanceID:    cfg.InstanceID,
		batchSize:     cfg.BatchSize,
		maxAttempts:   cfg.MaxAttempts,
		claimTTL:      cfg.ClaimTTL,
		pollInterval:  cfg.PollInterval,
	}
}

// Run starts the raw Kafka ingestion loop (non-blocking — it starts its own
// goroutine and returns immediately) and then blocks running its own poll
// ticker over the inbox table until ctx is cancelled. This is the
// entrypoint main.go calls directly, in place of the two separately-driven
// loops (libs/inbox.Consumer.Run + libs/inbox.Runner.Run) this type used to
// need wired up around it.
func (c *EmailReceivedConsumer) Run(ctx context.Context) error {
	if err := c.kafkaConsumer.Run(ctx); err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.PollOnce(ctx); err != nil {
				log.Error().Err(err).Msg("email-received consumer poll failed")
			}
		}
	}
}

// PollOnce claims a batch of pending inbox rows and processes each in turn.
// One bad record does not stop the rest of the batch. Exported so tests can
// drive a single pass directly without waiting on Run's ticker.
func (c *EmailReceivedConsumer) PollOnce(ctx context.Context) error {
	records, err := c.service.FetchBatch(ctx, c.topic, c.instanceID, c.batchSize, c.claimTTL)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	for _, rec := range records {
		log.Info().Str("topic", rec.Topic).Int64("inbox_id", rec.ID).Msg("received inbox record")
		c.processRecord(ctx, rec.ID, rec.Payload)
	}

	return nil
}

// processRecord decodes and processes a single record, recording a decode
// failure via MarkFailed rather than letting it abort the batch.
func (c *EmailReceivedConsumer) processRecord(ctx context.Context, recordID int64, payload []byte) {
	log := logger.FromContext(ctx)

	wire, err := c.decoder.Decode(ctx, payload)
	if err != nil {
		if markErr := c.service.MarkFailed(ctx, recordID, err, c.maxAttempts); markErr != nil {
			log.Error().Err(markErr).Int64("inbox_id", recordID).Msg("failed to mark inbox record failed after decode error")
		}
		return
	}

	event := domaininbox.EmailReceivedEvent{
		Subject:        wire.Subject,
		Body:           wire.Body,
		From:           wire.From,
		Date:           wire.Date,
		MessageID:      wire.MessageID,
		OrganizationID: wire.OrganizationID,
	}
	if err := c.service.ProcessEmailReceived(ctx, recordID, event, c.maxAttempts); err != nil {
		log.Error().Err(err).Int64("inbox_id", recordID).Msg("failed to process inbox record")
	}
}
