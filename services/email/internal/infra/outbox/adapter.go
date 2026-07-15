package outbox

import (
	"context"
	"time"

	libsoutbox "planeo/libs/outbox"
)

// Repository is implemented once per service (satisfied directly by
// *postgres.Client), shared by every producer adapter in the service.
// topic/instanceID are passed per call so one repository instance can
// serve any number of topic-scoped adapters.
type Repository interface {
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsoutbox.Record, error)
	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error
}

// EmailReceivedProducerAdapter owns the claim/produce/mark flow for the
// "email-received" topic. A service with more than one producer gets one
// adapter type per topic, not a single generic parameterized type.
type EmailReceivedProducerAdapter struct {
	repo        Repository
	producer    libsoutbox.Producer
	topic       string
	instanceID  string
	batchSize   int
	claimTTL    time.Duration
	maxAttempts int
}

func NewEmailReceivedProducerAdapter(
	repo Repository,
	producer libsoutbox.Producer,
	topic string,
	instanceID string,
	batchSize int,
	maxAttempts int,
	claimTTL time.Duration,
) *EmailReceivedProducerAdapter {
	return &EmailReceivedProducerAdapter{
		repo:        repo,
		producer:    producer,
		topic:       topic,
		instanceID:  instanceID,
		batchSize:   batchSize,
		claimTTL:    claimTTL,
		maxAttempts: maxAttempts,
	}
}

// PollOnce claims a batch of pending rows for this adapter's topic and
// produces each to Kafka, sequentially. No transaction wraps ProduceSync —
// Kafka isn't enrolled in Postgres's transaction, so wrapping it here would
// buy no atomicity; the known "produce succeeds, mark fails, resend on next
// poll" duplicate-send risk is unchanged from today's Relay.
func (a *EmailReceivedProducerAdapter) PollOnce(ctx context.Context) error {
	records, err := a.repo.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if err := a.producer.ProduceSync(ctx, rec.Topic, rec.Key, rec.Payload); err != nil {
			_ = a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
			continue
		}
		_ = a.repo.MarkProcessed(ctx, rec.ID)
	}

	return nil
}
