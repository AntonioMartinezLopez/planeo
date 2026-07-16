package inbox

import (
	"context"
	"time"

	"planeo/libs/logger"
)

const DefaultPollInterval = 1 * time.Second

// Runner calls a poll function on a fixed interval until ctx is cancelled.
// It has no knowledge of Store, Handler, or any claim/mark logic — those
// now live in services/core's own adapter, which supplies its own poll
// function to Run. Implemented separately from libs/outbox.Runner (same
// shape, not shared) — preserves the existing independently-implemented
// boundary between the two libs.
type Runner struct {
	pollInterval time.Duration
}

type Option func(*Runner)

func WithPollInterval(d time.Duration) Option {
	return func(r *Runner) { r.pollInterval = d }
}

func NewRunner(opts ...Option) *Runner {
	r := &Runner{pollInterval: DefaultPollInterval}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run calls poll on every tick until ctx is cancelled. It blocks the
// calling goroutine. poll's own errors are logged, not returned — a single
// failed poll must not stop the loop.
func (r *Runner) Run(ctx context.Context, poll func(context.Context) error) error {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := poll(ctx); err != nil {
				log.Error().Err(err).Msg("inbox runner poll failed")
			}
		}
	}
}
