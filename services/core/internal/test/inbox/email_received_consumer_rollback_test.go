package inbox_test

import (
	"context"
	"encoding/json"
	"planeo/libs/events/contracts"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/domain/inbox/mocks"
	"planeo/services/core/internal/domain/request"
	"planeo/services/core/internal/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestEmailReceivedConsumerServiceRollback exercises a real Postgres
// transaction rollback, which mocked-repository unit tests cannot reproduce:
// once a statement inside a transaction errors (here, a foreign key
// violation), Postgres aborts the whole transaction and every subsequent
// statement on it also fails (SQLSTATE 25P02). This guards against a
// regression where MarkFailed is accidentally called with the aborted
// transaction's context instead of the outer one.
func TestEmailReceivedConsumerServiceRollback(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)

	categoryService := category.NewService(env.DB)
	requestService := request.NewService(env.DB)

	mockLLMClient := mocks.NewMockLLMClient(t)
	mockLLMClient.EXPECT().
		ExtractRequestFields(mock.Anything, mock.Anything).
		Return(inbox.ExtractorOutput{}, nil)
	mockLLMClient.EXPECT().
		ClassifyRequest(mock.Anything, mock.Anything, mock.Anything).
		Return(0, nil)

	service := inbox.NewService(env.DB, requestService, categoryService, mockLLMClient)

	// organization 999999 does not exist, so CreateRequest's INSERT violates
	// the requests.organization_id foreign key and the transaction aborts.
	payload := contracts.EmailCreatedPayload{
		Subject:        "Broken organization reference",
		Body:           "body",
		From:           "someone@example.com",
		Date:           time.Now(),
		MessageID:      "rollback-test-1",
		OrganizationId: 999999,
	}
	payloadBytes, err := json.Marshal(payload)
	assert.Nil(t, err)

	_, err = env.DB.Save(context.Background(), "email-received", 0, 1, payloadBytes)
	assert.Nil(t, err)

	records, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(records))
	rec := records[0]

	// maxAttempts=2 so the failure resets the row to 'pending' (attempts=1 <
	// maxAttempts=2) rather than quarantining it. This is the only outcome
	// that can distinguish correct behavior from the regression: FetchBatch
	// lets ANY instance claim a 'pending' row regardless of claimed_by/TTL,
	// but a row stuck at 'processing' is only reclaimable by a different
	// instance once its claimTTL elapses. A quarantined ('failed') row would
	// be invisible to everyone either way, masking the distinction.
	err = service.ProcessEmailReceived(context.Background(), rec, 2)
	assert.Nil(t, err, "ProcessEmailReceived succeeds once MarkFailed successfully records the failure")

	// A different instance, well within the original claim's TTL, must see
	// the row - which only happens if MarkFailed reset it to 'pending' using
	// the outer (non-aborted) context. If MarkFailed had instead been called
	// with the aborted transaction's context, that call would itself fail,
	// leaving the row stuck at status='processing' under instance-a's claim -
	// invisible to a different instance until the TTL elapses.
	reclaimed, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-b", 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(reclaimed), "MarkFailed must reset the row to pending outside the aborted transaction")
}
