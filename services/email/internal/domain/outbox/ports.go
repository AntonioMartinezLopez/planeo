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

// Producer sends a single record to Kafka. Declared here (rather than
// consuming libsoutbox.Producer directly) so mockery can generate a mock for
// unit tests; libsoutbox.NewProducer's return value satisfies this interface
// directly, no wrapper needed.
type Producer interface {
	ProduceSync(ctx context.Context, topic string, key, value []byte) error
}

// Service owns the fetch/produce/mark resource handling for outbox messages.
// Exposed to the driving adapter (infra/outbox.EmailReceivedProducer), which
// owns the polling loop and topic/instance configuration.
type Service interface {
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsoutbox.Record, error)
	ProcessRecord(ctx context.Context, rec libsoutbox.Record, maxAttempts int) error
}
