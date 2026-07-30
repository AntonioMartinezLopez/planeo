package outbox

import (
	"context"
	"time"

	libsoutbox "planeo/libs/outbox"
)

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsoutbox.Record, error) {
	return s.repo.FetchBatch(ctx, topic, instanceID, limit, claimTTL)
}

func (s *service) MarkProcessed(ctx context.Context, id int64) error {
	return s.repo.MarkProcessed(ctx, id)
}

func (s *service) MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error {
	return s.repo.MarkFailed(ctx, id, sendErr, maxAttempts)
}
