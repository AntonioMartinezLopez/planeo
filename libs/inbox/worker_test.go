package inbox_test

import (
	"context"
	"errors"
	"planeo/libs/inbox"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeStore struct {
	mu         sync.Mutex
	records    []inbox.Record
	processed  []int64
	failed     map[int64]int
	maxReached []int64
}

func newFakeStore(records []inbox.Record) *fakeStore {
	return &fakeStore{records: records, failed: map[int64]int{}}
}

func (f *fakeStore) Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error) {
	return true, nil
}

func (f *fakeStore) FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]inbox.Record, error) {
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

func (f *fakeStore) MarkProcessed(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed = append(f.processed, id)
	return nil
}

func (f *fakeStore) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed[id]++
	if f.failed[id] >= maxAttempts {
		f.maxReached = append(f.maxReached, id)
	}
	return nil
}

func TestWorker(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("processes a fetched record and marks it processed", func(t *testing.T) {
		store := newFakeStore([]inbox.Record{{ID: 1, Topic: "t", Payload: []byte("v")}})
		var handled []int64
		handler := func(ctx context.Context, rec inbox.Record) error {
			handled = append(handled, rec.ID)
			return nil
		}
		worker := inbox.NewWorker(store, handler, inbox.WithPollInterval(10*time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = worker.Run(ctx)

		assert.Equal(t, []int64{1}, handled)
		assert.Equal(t, []int64{1}, store.processed)
	})

	t.Run("marks a record failed and quarantines it after max attempts", func(t *testing.T) {
		record := inbox.Record{ID: 2, Topic: "broken-topic", Payload: []byte("v")}
		store := newFakeStore([]inbox.Record{record, record, record})
		handler := func(ctx context.Context, rec inbox.Record) error {
			return errors.New("simulated handler failure")
		}
		worker := inbox.NewWorker(store, handler, inbox.WithPollInterval(10*time.Millisecond), inbox.WithMaxAttempts(2))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = worker.Run(ctx)

		assert.GreaterOrEqual(t, store.failed[2], 2)
		assert.Contains(t, store.maxReached, int64(2))
	})
}
