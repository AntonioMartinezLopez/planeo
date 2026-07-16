package inbox

import (
	"context"
	"time"

	libsinbox "planeo/libs/inbox"
	"planeo/services/core/internal/domain/category"
)

// Repository is implemented once per service (satisfied directly by
// *postgres.Client). topic scopes FetchBatch so that if services/core's
// inbox ever receives a second topic, two consumers can't steal each
// other's rows — mirroring the producer-side outbox.Repository.
type Repository interface {
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error)
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error
}

// LLMClient extracts structured fields from a request and classifies it into
// a category. Declared here so mockery can generate a mock for unit tests
// that don't hit a live Mistral API.
type LLMClient interface {
	ExtractRequestFields(ctx context.Context, requestText string) (ExtractorOutput, error)
	ClassifyRequest(ctx context.Context, request RequestData, categories []category.Category) (int, error)
}

// Service owns the fetch/process/mark resource handling for inbox messages.
// Exposed to the driving adapter (infra/inbox.EmailReceivedConsumer), which
// owns the polling loop and topic/instance configuration. Processing methods
// are named per topic (ProcessEmailReceived, not a generic ProcessRecord)
// since a future second topic would need its own dedicated payload handling,
// not a shared one.
type Service interface {
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error)
	ProcessEmailReceived(ctx context.Context, rec libsinbox.Record, maxAttempts int) error
}
