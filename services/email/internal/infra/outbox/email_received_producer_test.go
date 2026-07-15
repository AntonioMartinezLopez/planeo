package outbox_test

import (
	"context"
	"errors"
	libsoutbox "planeo/libs/outbox"
	"planeo/services/email/internal/infra/outbox"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeRepository struct {
	mu         sync.Mutex
	records    []libsoutbox.Record
	processed  []int64
	failed     map[int64]int
	maxReached []int64
}

func newFakeRepository(records []libsoutbox.Record) *fakeRepository {
	return &fakeRepository{records: records, failed: map[int64]int{}}
}

func (f *fakeRepository) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsoutbox.Record, error) {
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

func (f *fakeRepository) MarkProcessed(ctx context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processed = append(f.processed, id)
	return nil
}

func (f *fakeRepository) MarkFailed(ctx context.Context, id int64, sendErr error, maxAttempts int) error {
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
	sent      []libsoutbox.Record
	failTopic string
}

func (f *fakeProducer) ProduceSync(ctx context.Context, topic string, key, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if topic == f.failTopic {
		return errors.New("simulated produce failure")
	}
	f.sent = append(f.sent, libsoutbox.Record{Topic: topic, Key: key, Payload: value})
	return nil
}

func TestEmailReceivedProducer(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("produces a fetched record and marks it processed", func(t *testing.T) {
		repo := newFakeRepository([]libsoutbox.Record{{ID: 1, Topic: "email-received", Key: []byte("k"), Payload: []byte("v")}})
		producer := &fakeProducer{}
		emailReceivedProducer := outbox.NewEmailReceivedProducer(repo, producer, "email-received", "instance-a", 10, 5, 30*time.Second)

		err := emailReceivedProducer.PollOnce(context.Background())

		assert.Nil(t, err)
		assert.Equal(t, 1, len(producer.sent))
		assert.Equal(t, []int64{1}, repo.processed)
	})

	t.Run("marks a record failed and quarantines it after max attempts", func(t *testing.T) {
		record := libsoutbox.Record{ID: 2, Topic: "broken-topic", Key: nil, Payload: []byte("v")}
		repo := newFakeRepository([]libsoutbox.Record{record, record, record})
		producer := &fakeProducer{failTopic: "broken-topic"}
		emailReceivedProducer := outbox.NewEmailReceivedProducer(repo, producer, "broken-topic", "instance-a", 1, 2, 30*time.Second)

		assert.Nil(t, emailReceivedProducer.PollOnce(context.Background()))
		assert.Nil(t, emailReceivedProducer.PollOnce(context.Background()))
		assert.Nil(t, emailReceivedProducer.PollOnce(context.Background()))

		assert.Equal(t, 0, len(producer.sent))
		assert.GreaterOrEqual(t, repo.failed[2], 2)
		assert.Contains(t, repo.maxReached, int64(2))
	})
}
