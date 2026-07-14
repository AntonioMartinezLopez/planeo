package inbox

import (
	"context"
	"time"
)

// Record is one durably-persisted inbox row, ready for processing.
type Record struct {
	ID      int64
	Topic   string
	Payload []byte
}

// Store is implemented per-service against that service's own inbox table.
// Mirrors outbox.Store's shape, for the opposite direction.
type Store interface {
	// Save durably persists one raw Kafka record, deduped on
	// (topic, partition, offset). inserted=false on a duplicate — a safe
	// no-op, not an error. The caller commits the Kafka offset regardless
	// of inserted's value, since either way this offset is now durably
	// known.
	Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (inserted bool, err error)

	// FetchBatch atomically claims up to limit pending (or expired-claim)
	// records in insertion order. Same FOR UPDATE SKIP LOCKED
	// atomic-UPDATE-with-inner-SELECT requirement as outbox.Store.
	FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]Record, error)

	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error
}
