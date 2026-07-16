package inbox

import (
	"context"
	"time"

	"planeo/libs/logger"
	domaininbox "planeo/services/core/internal/domain/inbox"
)

// EmailReceivedConsumer owns the polling loop for the "email-received"
// topic: it fetches a batch of claimed rows and hands each one to the
// domain service for the actual extract/classify/create resource handling.
type EmailReceivedConsumer struct {
	service     domaininbox.Service
	topic       string
	instanceID  string
	batchSize   int
	claimTTL    time.Duration
	maxAttempts int
}

func NewEmailReceivedConsumer(
	service domaininbox.Service,
	topic string,
	instanceID string,
	batchSize int,
	maxAttempts int,
	claimTTL time.Duration,
) *EmailReceivedConsumer {
	return &EmailReceivedConsumer{
		service:     service,
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
		if err := a.service.ProcessEmailReceived(ctx, rec, a.maxAttempts); err != nil {
			log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to process inbox record")
		}
	}

	return nil
}
