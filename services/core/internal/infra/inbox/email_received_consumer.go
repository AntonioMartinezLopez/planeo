package inbox

import (
	"context"
	"time"

	"planeo/libs/eventschemas"
	"planeo/libs/logger"
	"planeo/libs/schemaregistry"
	domaininbox "planeo/services/core/internal/domain/inbox"
)

// EmailReceivedConsumer owns the polling loop for the "email-received"
// topic: it fetches a batch of claimed rows, Avro-decodes each payload
// (this is the coupling site described in the design spec's Section 3),
// and hands the decoded event to the domain service. serde is already
// registered for EmailReceived by the time it reaches here (see
// cmd/main.go, Step 6) — this package never imports franz-go/pkg/sr, and
// this constructor is plain, matching every other infra adapter
// constructor in this codebase.
type EmailReceivedConsumer struct {
	service     domaininbox.Service
	serde       *schemaregistry.Serde
	topic       string
	instanceID  string
	batchSize   int
	claimTTL    time.Duration
	maxAttempts int
}

func NewEmailReceivedConsumer(
	service domaininbox.Service,
	serde *schemaregistry.Serde,
	topic string,
	instanceID string,
	batchSize int,
	maxAttempts int,
	claimTTL time.Duration,
) *EmailReceivedConsumer {
	return &EmailReceivedConsumer{
		service:     service,
		serde:       serde,
		topic:       topic,
		instanceID:  instanceID,
		batchSize:   batchSize,
		claimTTL:    claimTTL,
		maxAttempts: maxAttempts,
	}
}

// PollOnce claims a batch of pending inbox rows and processes each in turn.
// One bad record does not stop the rest of the batch.
func (a *EmailReceivedConsumer) PollOnce(ctx context.Context) error {
	records, err := a.service.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	for _, rec := range records {
		log.Info().Str("topic", rec.Topic).Int64("inbox_id", rec.ID).Msg("received inbox record")

		var wire eventschemas.EmailReceived
		if err := schemaregistry.Decode(ctx, a.serde, rec.Payload, eventschemas.EmailReceivedSchema, &wire); err != nil {
			if markErr := a.service.MarkFailed(ctx, rec.ID, err, a.maxAttempts); markErr != nil {
				log.Error().Err(markErr).Int64("inbox_id", rec.ID).Msg("failed to mark inbox record failed after decode error")
			}
			continue
		}

		event := domaininbox.EmailReceivedEvent{
			Subject:        wire.Subject,
			Body:           wire.Body,
			From:           wire.From,
			Date:           wire.Date,
			MessageID:      wire.MessageID,
			OrganizationID: wire.OrganizationID,
		}
		if err := a.service.ProcessEmailReceived(ctx, rec.ID, event, a.maxAttempts); err != nil {
			log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to process inbox record")
		}
	}

	return nil
}
