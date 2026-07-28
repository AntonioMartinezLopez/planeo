package outbox

import (
	"context"
	"time"

	libsoutbox "planeo/libs/outbox"
)

// Repository is implemented once per service (satisfied directly by
// *postgres.Client), shared by every producer in the service. topic/instanceID
// are passed per call so one repository instance can serve any number of
// topic-scoped producers.
type Repository interface {
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsoutbox.Record, error)
	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error
}

// Service owns the fetch/mark resource handling for outbox messages.
// Exposed to the driving adapter (infra/outbox.EmailReceivedProducer), which
// owns the polling loop, the actual Kafka send, and Avro encoding — this
// service no longer knows a message is ever produced, only that a batch can
// be fetched and a record's outcome recorded.
type Service interface {
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsoutbox.Record, error)
	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error
}
