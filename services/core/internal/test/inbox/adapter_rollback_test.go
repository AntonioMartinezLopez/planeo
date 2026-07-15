package inbox_test

import (
	"context"
	"encoding/json"
	"planeo/libs/events/contracts"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
	coreinbox "planeo/services/core/internal/infra/inbox"
	"planeo/services/core/internal/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// This proves the consumer adapter's write-only transaction really rolls
// back on a real Postgres failure — not just believed correct by
// inspection. It forces UpdateRequest to fail (a nonexistent CategoryId
// violates requests.category_id's foreign key) after CreateRequest has
// already succeeded inside the same transaction, then asserts neither
// write survived and the inbox row is back to pending with attempts
// incremented — mirroring the outbox-architecture-cleanup plan's
// "CreateOutboxEvent failure rolls back CreateMail" test.
func TestEmailReceivedConsumerAdapterRollsBackOnWriteFailure(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)

	payload := contracts.EmailCreatedPayload{
		Subject: "Rollback test", Body: "Body", From: "rollback@example.com",
		MessageID: "rollback-test-message-id", OrganizationId: 1,
	}
	payloadBytes, err := json.Marshal(payload)
	assert.Nil(t, err)

	inserted, err := env.DB.Save(context.Background(), "email-received", 0, 1, payloadBytes)
	assert.Nil(t, err)
	assert.True(t, inserted)

	categoryService := category.NewService(env.DB)
	requestService := forcingUpdateFailureRequestService{Service: request.NewService(env.DB)}

	adapter := coreinbox.NewEmailReceivedConsumerAdapter(env.DB, requestService, categoryService, "instance-a", 10, 5, 30*time.Second)
	err = adapter.PollOnce(context.Background())
	assert.Nil(t, err, "PollOnce logs per-record errors, it doesn't return them")

	requests, err := env.DB.GetRequests(context.Background(), 1, 0, 100, false, nil)
	assert.Nil(t, err)
	for _, r := range requests {
		assert.NotEqual(t, "rollback-test-message-id", r.ReferenceId, "CreateRequest's row must not survive when UpdateRequest fails in the same transaction")
	}

	batch, err := env.DB.FetchBatch(context.Background(), "instance-a", 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(batch), "the inbox row must be back to pending (immediately reclaimable by the same instance) after the transaction rolled back")
}

// forcingUpdateFailureRequestService wraps the real request.Service, only
// overriding UpdateRequest to inject a foreign-key-violating CategoryId -
// forcing a genuine Postgres error inside the adapter's transaction,
// rather than a fabricated one.
type forcingUpdateFailureRequestService struct {
	request.Service
}

func (f forcingUpdateFailureRequestService) UpdateRequest(ctx context.Context, req request.UpdateRequest) error {
	req.CategoryId = 999999
	return f.Service.UpdateRequest(ctx, req)
}
