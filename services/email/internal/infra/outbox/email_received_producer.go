package outbox

import (
	"context"
	"time"

	"planeo/libs/logger"
	domainoutbox "planeo/services/email/internal/domain/outbox"
)

// EmailReceivedProducer owns the polling loop for the "email-received"
// topic: it fetches a batch of claimed rows and hands each one to the
// domain service for the actual produce/mark resource handling. A service
// with more than one producer gets one adapter type per topic, not a single
// generic parameterized type.
type EmailReceivedProducer struct {
	service     domainoutbox.Service
	topic       string
	instanceID  string
	batchSize   int
	claimTTL    time.Duration
	maxAttempts int
}

func NewEmailReceivedProducer(
	service domainoutbox.Service,
	topic string,
	instanceID string,
	batchSize int,
	maxAttempts int,
	claimTTL time.Duration,
) *EmailReceivedProducer {
	return &EmailReceivedProducer{
		service:     service,
		topic:       topic,
		instanceID:  instanceID,
		batchSize:   batchSize,
		claimTTL:    claimTTL,
		maxAttempts: maxAttempts,
	}
}

// PollOnce claims a batch of pending rows for this producer's topic and
// processes each in turn, sequentially.
func (a *EmailReceivedProducer) PollOnce(ctx context.Context) error {
	records, err := a.service.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	for _, rec := range records {
		log.Info().Str("topic", rec.Topic).Int64("outbox_id", rec.ID).Msg("sending outbox record")
		_ = a.service.ProcessRecord(ctx, rec, a.maxAttempts)
	}

	return nil
}
