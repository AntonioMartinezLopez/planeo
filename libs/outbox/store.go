package outbox

import (
	"context"
	"time"
)

// Record is a single outbox row ready to be produced to Kafka: already
// fully serialized bytes, opaque to the relay.
type Record struct {
	ID      int64
	Topic   string
	Key     []byte
	Payload []byte
}

// Store is implemented per-service against that service's own outbox
// table. This is the extension point for future services: business-
// specific fetch/claim/mark logic all lives in a Store implementation,
// never in Relay itself.
type Store interface {
	// FetchBatch atomically claims up to limit records that are either
	// pending or whose previous claim has expired (claimed longer ago than
	// claimTTL), and returns them for producing. This must be a single
	// atomic statement (e.g. UPDATE ... WHERE id IN (SELECT ... FOR UPDATE
	// SKIP LOCKED) RETURNING ...), not a separate SELECT followed later by
	// a separate mark call — otherwise the claim provides no protection
	// against a concurrent second poller. The FOR UPDATE SKIP LOCKED must
	// live in the inner SELECT, not be assumed implicit from the outer
	// UPDATE's WHERE id IN (...) alone — the latter does not reliably
	// prevent two concurrent claims from selecting overlapping ids.
	FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]Record, error)

	// MarkProcessed marks a record as successfully sent.
	MarkProcessed(ctx context.Context, id int64) error

	// MarkFailed records a failed send attempt. If the resulting attempt
	// count is still below maxAttempts, the record must be reset so it's
	// eligible for the next FetchBatch (not left claimed). Once attempts
	// reaches maxAttempts, the record is quarantined: excluded from future
	// FetchBatch calls, retained for inspection.
	MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error
}
