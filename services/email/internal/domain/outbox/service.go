package outbox

import (
	"context"
	"time"

	libsoutbox "planeo/libs/outbox"
)

type service struct {
	repo     Repository
	producer Producer
}

func NewService(repo Repository, producer Producer) Service {
	return &service{repo: repo, producer: producer}
}

func (s *service) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsoutbox.Record, error) {
	return s.repo.FetchBatch(ctx, topic, instanceID, limit, claimTTL)
}

// ProcessRecord produces a single record to Kafka and marks it accordingly.
// No transaction wraps ProduceSync — Kafka isn't enrolled in Postgres's
// transaction, so wrapping it here would buy no atomicity; the known
// "produce succeeds, mark fails, resend on next poll" duplicate-send risk is
// unchanged from the pre-domain-service design.
func (s *service) ProcessRecord(ctx context.Context, rec libsoutbox.Record, maxAttempts int) error {
	if err := s.producer.ProduceSync(ctx, rec.Topic, rec.Key, rec.Payload); err != nil {
		return s.repo.MarkFailed(ctx, rec.ID, err, maxAttempts)
	}
	return s.repo.MarkProcessed(ctx, rec.ID)
}
