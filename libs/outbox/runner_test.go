package outbox_test

import (
	"context"
	"errors"
	"planeo/libs/outbox"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunner(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("calls poll on every tick until ctx is cancelled", func(t *testing.T) {
		var calls int32
		poll := func(ctx context.Context) error {
			atomic.AddInt32(&calls, 1)
			return nil
		}
		runner := outbox.NewRunner(outbox.WithPollInterval(10 * time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Millisecond)
		defer cancel()
		err := runner.Run(ctx, poll)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(4))
	})

	t.Run("a poll error does not stop the loop", func(t *testing.T) {
		var calls int32
		poll := func(ctx context.Context) error {
			atomic.AddInt32(&calls, 1)
			return errors.New("simulated poll failure")
		}
		runner := outbox.NewRunner(outbox.WithPollInterval(10 * time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
		defer cancel()
		_ = runner.Run(ctx, poll)

		assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2))
	})
}
