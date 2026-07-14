package inbox

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

// Handler processes one durably-persisted inbox record. Injected by the
// consuming service — this is where business logic (calling domain
// services, LLM, etc.) lives. Never called until the record is already
// safely persisted.
type Handler func(ctx context.Context, record Record) error

type Worker struct {
	store        Store
	handler      Handler
	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	claimTTL     time.Duration
}

type Option func(*Worker)

func WithPollInterval(d time.Duration) Option {
	return func(w *Worker) { w.pollInterval = d }
}

func WithBatchSize(n int) Option {
	return func(w *Worker) { w.batchSize = n }
}

func WithMaxAttempts(n int) Option {
	return func(w *Worker) { w.maxAttempts = n }
}

func WithClaimTTL(d time.Duration) Option {
	return func(w *Worker) { w.claimTTL = d }
}

func NewWorker(store Store, handler Handler, opts ...Option) *Worker {
	w := &Worker{
		store:        store,
		handler:      handler,
		pollInterval: DefaultPollInterval,
		batchSize:    DefaultBatchSize,
		maxAttempts:  DefaultMaxAttempts,
		claimTTL:     DefaultClaimTTL,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Run polls the inbox and invokes Handler for each claimed record,
// sequentially, until ctx is cancelled. It blocks the calling goroutine.
func (w *Worker) Run(ctx context.Context) error {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.pollOnce(ctx, log); err != nil {
				log.Error().Err(err).Msg("inbox worker poll failed")
			}
		}
	}
}

func (w *Worker) pollOnce(ctx context.Context, log zerolog.Logger) error {
	records, err := w.store.FetchBatch(ctx, w.batchSize, w.claimTTL)
	if err != nil {
		return err
	}

	for _, rec := range records {
		if err := w.handler(ctx, rec); err != nil {
			log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to process inbox record")
			if markErr := w.store.MarkFailed(ctx, rec.ID, err, w.maxAttempts); markErr != nil {
				log.Error().Err(markErr).Int64("inbox_id", rec.ID).Msg("failed to mark inbox record as failed")
			}
			continue
		}

		if err := w.store.MarkProcessed(ctx, rec.ID); err != nil {
			log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to mark inbox record as processed")
		}
	}

	return nil
}
