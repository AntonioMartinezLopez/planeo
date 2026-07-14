package outbox_test

import (
	"context"
	"errors"
	"planeo/libs/outbox"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeStore struct {
	mu         sync.Mutex
	records    []outbox.Record
	processed  []int64
	failed     map[int64]int
	maxReached []int64
}

func newFakeStore(records []outbox.Record) *fakeStore {
	return &fakeStore{records: records, failed: map[int64]int{}}
}

func (f *fakeStore) FetchBatch(ctx context.Context, limit int, claimTTL time.Duration) ([]outbox.Record, error) {
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

func (f *fakeStore) MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed[id]++
	if f.failed[id] >= maxAttempts {
		f.maxReached = append(f.maxReached, id)
	}
	return nil
}

type fakeProducer struct {
	mu        sync.Mutex
	sent      []outbox.Record
	failTopic string
}

func (f *fakeProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if topic == f.failTopic {
		return errors.New("simulated produce failure")
	}
	f.sent = append(f.sent, outbox.Record{Topic: topic, Key: key, Payload: value})
	return nil
}

func TestRelay(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("produces a fetched record and marks it processed", func(t *testing.T) {
		store := newFakeStore([]outbox.Record{{ID: 1, Topic: "t", Key: []byte("k"), Payload: []byte("v")}})
		producer := &fakeProducer{}
		relay := outbox.NewRelay(store, producer, outbox.WithPollInterval(10*time.Millisecond))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = relay.Run(ctx)

		assert.Equal(t, 1, len(producer.sent))
		assert.Equal(t, []int64{1}, store.processed)
	})

	t.Run("marks a record failed and quarantines it after max attempts", func(t *testing.T) {
		// Simulates 3 poll cycles' worth of fetches for the same still-
		// unprocessed row by queuing 3 copies of it up front — a
		// simplification of a real Store, which would keep returning the
		// same unprocessed row across polls until it succeeds or is
		// quarantined.
		record := outbox.Record{ID: 2, Topic: "broken-topic", Key: nil, Payload: []byte("v")}
		store := newFakeStore([]outbox.Record{record, record, record})
		producer := &fakeProducer{failTopic: "broken-topic"}
		relay := outbox.NewRelay(store, producer, outbox.WithPollInterval(10*time.Millisecond), outbox.WithMaxAttempts(2))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = relay.Run(ctx)

		assert.Equal(t, 0, len(producer.sent))
		assert.GreaterOrEqual(t, store.failed[2], 2)
		assert.Contains(t, store.maxReached, int64(2))
	})
}
