package inbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"planeo/libs/events/contracts"
	libsinbox "planeo/libs/inbox"
	"planeo/services/core/internal/domain/category"
	categorymocks "planeo/services/core/internal/domain/category/mocks"
	requestmocks "planeo/services/core/internal/domain/request/mocks"
	"planeo/services/core/internal/infra/inbox"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeRepository struct {
	mu          sync.Mutex
	records     []libsinbox.Record
	processed   []int64
	failed      map[int64]int
	txShouldErr bool
}

func newFakeRepository(records []libsinbox.Record) *fakeRepository {
	return &fakeRepository{records: records, failed: map[int64]int{}}
}

func (f *fakeRepository) FetchBatch(ctx context.Context, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.records) == 0 {
		return nil, nil
	}
	n := limit
	if n > len(f.records) {
		n = len(f.records)
	}
	batch := f.records[:n]
	f.records = f.records[n:]
	return batch, nil
}

func (f *fakeRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if f.txShouldErr {
		return errors.New("simulated transaction failure")
	}
	return fn(ctx)
}

func (f *fakeRepository) MarkProcessed(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed = append(f.processed, id)
	return nil
}

func (f *fakeRepository) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed[id]++
	return nil
}

func mustPayload(t *testing.T, payload contracts.EmailCreatedPayload) []byte {
	t.Helper()
	b, err := json.Marshal(payload)
	assert.Nil(t, err)
	return b
}

func TestEmailReceivedConsumerAdapter(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	payload := mustPayload(t, contracts.EmailCreatedPayload{
		Subject: "Subject", Body: "Body", From: "sender@example.com",
		MessageID: "msg-1", OrganizationId: 1,
	})

	t.Run("processes a record and marks it processed", func(t *testing.T) {
		repo := newFakeRepository([]libsinbox.Record{{ID: 1, Topic: "email-received", Payload: payload}})
		requestService := requestmocks.NewMockService(t)
		categoryService := categorymocks.NewMockService(t)

		categoryService.EXPECT().GetCategories(mock.Anything, 1).Return([]category.Category{}, nil)
		requestService.EXPECT().CreateRequest(mock.Anything, mock.Anything).Return(42, nil)
		requestService.EXPECT().UpdateRequest(mock.Anything, mock.Anything).Return(nil)

		adapter := inbox.NewEmailReceivedConsumerAdapter(repo, requestService, categoryService, "instance-a", 10, 5, 30*time.Second)
		err := adapter.PollOnce(context.Background())

		assert.Nil(t, err)
		assert.Equal(t, []int64{1}, repo.processed)
	})

	t.Run("a transaction failure calls MarkFailed on the plain ctx, after rollback", func(t *testing.T) {
		repo := newFakeRepository([]libsinbox.Record{{ID: 2, Topic: "email-received", Payload: payload}})
		repo.txShouldErr = true
		requestService := requestmocks.NewMockService(t)
		categoryService := categorymocks.NewMockService(t)

		categoryService.EXPECT().GetCategories(mock.Anything, 1).Return([]category.Category{}, nil)

		adapter := inbox.NewEmailReceivedConsumerAdapter(repo, requestService, categoryService, "instance-a", 10, 5, 30*time.Second)
		err := adapter.PollOnce(context.Background())

		assert.Nil(t, err, "PollOnce logs per-record errors, it doesn't return them")
		assert.Equal(t, 0, len(repo.processed))
		assert.Equal(t, 1, repo.failed[2])
	})
}
