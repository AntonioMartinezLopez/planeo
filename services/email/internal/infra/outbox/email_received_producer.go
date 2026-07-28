package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"planeo/libs/eventschemas"
	"planeo/libs/logger"
	libsoutbox "planeo/libs/outbox"
	domainoutbox "planeo/services/email/internal/domain/outbox"
)

// Encoder Avro-encodes a decoded EmailReceived value. Declared here (rather
// than depending on *schemaregistry.Serde directly) so it can be mocked in
// unit tests without a live registry; *schemaregistry.Encoder[eventschemas.EmailReceived]
// satisfies this structurally, no wrapper needed.
type Encoder interface {
	Encode(v eventschemas.EmailReceived) ([]byte, error)
}

// Deps are EmailReceivedProducer's collaborators: the domain service (fetch
// pending rows, record each outcome), the raw Kafka sender, and the Avro
// encoder. Grouped separately from Config so call sites read as "these are
// the things it talks to" vs "these are the knobs it runs with".
type Deps struct {
	Service  domainoutbox.Service
	Producer libsoutbox.Producer
	Encoder  Encoder
}

// Config is EmailReceivedProducer's scalar, topic/instance-specific
// configuration.
type Config struct {
	Topic        string
	InstanceID   string
	BatchSize    int
	MaxAttempts  int
	ClaimTTL     time.Duration
	PollInterval time.Duration
}

// EmailReceivedProducer owns everything the "email-received" outbox needs on
// the produce side: polling the domain service for a batch of pending rows,
// Avro-encoding each row's JSON-staged payload, sending it to Kafka, and
// recording the outcome — one type, no separate driving-adapter/encoding-
// wrapper split. The domain service (services/email/internal/domain/outbox)
// stays fully unaware a message is ever produced; it only fetches batches
// and records outcomes.
type EmailReceivedProducer struct {
	service      domainoutbox.Service
	producer     libsoutbox.Producer
	encoder      Encoder
	topic        string
	instanceID   string
	batchSize    int
	maxAttempts  int
	claimTTL     time.Duration
	pollInterval time.Duration
}

func NewEmailReceivedProducer(deps Deps, cfg Config) *EmailReceivedProducer {
	return &EmailReceivedProducer{
		service:      deps.Service,
		producer:     deps.Producer,
		encoder:      deps.Encoder,
		topic:        cfg.Topic,
		instanceID:   cfg.InstanceID,
		batchSize:    cfg.BatchSize,
		maxAttempts:  cfg.MaxAttempts,
		claimTTL:     cfg.ClaimTTL,
		pollInterval: cfg.PollInterval,
	}
}

// Run polls on a fixed interval until ctx is cancelled. It blocks the
// calling goroutine — this is the entrypoint main.go calls directly, in
// place of the generic libs/outbox.Runner this type used to be driven by. A
// single failed poll is logged, not fatal — it must not stop the loop.
func (p *EmailReceivedProducer) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.PollOnce(ctx); err != nil {
				log.Error().Err(err).Msg("email-received producer poll failed")
			}
		}
	}
}

// PollOnce claims a batch of pending rows and processes each in turn. One
// bad record does not stop the rest of the batch. Exported so tests can
// drive a single pass directly without waiting on Run's ticker.
func (p *EmailReceivedProducer) PollOnce(ctx context.Context) error {
	records, err := p.service.FetchBatch(ctx, p.topic, p.instanceID, p.batchSize, p.claimTTL)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	for _, rec := range records {
		log.Info().Str("topic", rec.Topic).Int64("outbox_id", rec.ID).Msg("sending outbox record")
		p.processRecord(ctx, rec)
	}

	return nil
}

// processRecord encodes and sends a single record, then records the
// outcome. No transaction wraps the send — Kafka isn't enrolled in
// Postgres's transaction, so wrapping it here would buy no atomicity; the
// known "send succeeds, mark fails, resend on next poll" duplicate-send
// risk is unchanged from the pre-merge design.
func (p *EmailReceivedProducer) processRecord(ctx context.Context, rec libsoutbox.Record) {
	log := logger.FromContext(ctx)
	if err := p.produce(ctx, rec); err != nil {
		if markErr := p.service.MarkFailed(ctx, rec.ID, err, p.maxAttempts); markErr != nil {
			log.Error().Err(markErr).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record failed after send error")
		}
		return
	}
	if err := p.service.MarkProcessed(ctx, rec.ID); err != nil {
		log.Error().Err(err).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record processed")
	}
}

// produce decodes the JSON-staged payload, Avro-encodes it, and sends it to
// Kafka under the record's own topic/key.
func (p *EmailReceivedProducer) produce(ctx context.Context, rec libsoutbox.Record) error {
	var wire eventschemas.EmailReceived
	if err := json.Unmarshal(rec.Payload, &wire); err != nil {
		return fmt.Errorf("decode staged outbox payload: %w", err)
	}

	avroBytes, err := p.encoder.Encode(wire)
	if err != nil {
		return fmt.Errorf("avro encode: %w", err)
	}

	return p.producer.ProduceSync(ctx, rec.Topic, rec.Key, avroBytes)
}
