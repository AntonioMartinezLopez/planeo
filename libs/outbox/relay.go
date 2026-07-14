package outbox

import (
	"context"
	"time"

	"planeo/libs/logger"

	"github.com/rs/zerolog"
)

const (
	DefaultPollInterval = 1 * time.Second
	DefaultBatchSize    = 100
	DefaultMaxAttempts  = 5
	DefaultClaimTTL     = 30 * time.Second
)

type Relay struct {
	store        Store
	producer     Producer
	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	claimTTL     time.Duration
}

type Option func(*Relay)

func WithPollInterval(d time.Duration) Option {
	return func(r *Relay) { r.pollInterval = d }
}

func WithBatchSize(n int) Option {
	return func(r *Relay) { r.batchSize = n }
}

func WithMaxAttempts(n int) Option {
	return func(r *Relay) { r.maxAttempts = n }
}

func WithClaimTTL(d time.Duration) Option {
	return func(r *Relay) { r.claimTTL = d }
}

func NewRelay(store Store, producer Producer, opts ...Option) *Relay {
	r := &Relay{
		store:        store,
		producer:     producer,
		pollInterval: DefaultPollInterval,
		batchSize:    DefaultBatchSize,
		maxAttempts:  DefaultMaxAttempts,
		claimTTL:     DefaultClaimTTL,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run polls the store and produces each claimed record to Kafka,
// sequentially, until ctx is cancelled. It blocks the calling goroutine.
func (r *Relay) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.pollOnce(ctx, log); err != nil {
				log.Error().Err(err).Msg("outbox relay poll failed")
			}
		}
	}
}

func (r *Relay) pollOnce(ctx context.Context, log zerolog.Logger) error {
	records, err := r.store.FetchBatch(ctx, r.batchSize, r.claimTTL)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if err := r.producer.ProduceSync(ctx, rec.Topic, rec.Key, rec.Payload); err != nil {
			log.Error().Err(err).Int64("outbox_id", rec.ID).Msg("failed to produce outbox record")
			if markErr := r.store.MarkFailed(ctx, rec.ID, err, r.maxAttempts); markErr != nil {
				log.Error().Err(markErr).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record as failed")
			}
			continue
		}

		if err := r.store.MarkProcessed(ctx, rec.ID); err != nil {
			log.Error().Err(err).Int64("outbox_id", rec.ID).Msg("failed to mark outbox record as processed")
		}
	}

	return nil
}
