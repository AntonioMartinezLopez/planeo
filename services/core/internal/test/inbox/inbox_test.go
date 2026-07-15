package inbox_test

import (
	"context"
	"errors"
	"planeo/services/core/internal/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInboxRepositorySave(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)

	t.Run("inserts a new record", func(t *testing.T) {
		inserted, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
		assert.Nil(t, err)
		assert.True(t, inserted)
	})

	t.Run("is idempotent on a duplicate topic+partition+offset", func(t *testing.T) {
		inserted, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
		assert.Nil(t, err)
		assert.False(t, inserted, "a conflicting (topic, partition, offset) must not create a second row")
	})
}

func TestInboxRepositoryFetchBatch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)
	_, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
	assert.Nil(t, err)

	t.Run("claims a pending record", func(t *testing.T) {
		records, err := env.DB.FetchBatch(context.Background(), "email-received", "test-instance", 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(records))
		assert.Equal(t, "email-received", records[0].Topic)
	})

	t.Run("does not reclaim a record still within its claim TTL", func(t *testing.T) {
		records, err := env.DB.FetchBatch(context.Background(), "email-received", "other-instance", 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(records), "the record claimed in the previous poll is still within its TTL")
	})

	t.Run("reclaims a record whose claim has expired", func(t *testing.T) {
		records, err := env.DB.FetchBatch(context.Background(), "email-received", "test-instance", 10, 0*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(records), "a claimTTL of 0 means any processing record is immediately reclaimable")
	})

	t.Run("MarkProcessed marks the record processed and excludes it from future batches", func(t *testing.T) {
		err := env.DB.MarkProcessed(context.Background(), 1)
		assert.Nil(t, err)

		records, err := env.DB.FetchBatch(context.Background(), "email-received", "test-instance", 10, 0*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(records))
	})
}

func TestInboxRepositoryMarkFailed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)
	_, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
	assert.Nil(t, err)

	records, err := env.DB.FetchBatch(context.Background(), "email-received", "test-instance", 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(records))
	recordID := records[0].ID

	t.Run("resets to pending when attempts is still below maxAttempts", func(t *testing.T) {
		err := env.DB.MarkFailed(context.Background(), recordID, errors.New("boom"), 3)
		assert.Nil(t, err)

		batch, err := env.DB.FetchBatch(context.Background(), "email-received", "test-instance", 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(batch), "a not-yet-exhausted failure must be reset to pending, not left claimed")
	})

	t.Run("quarantines the record once maxAttempts is reached", func(t *testing.T) {
		err := env.DB.MarkFailed(context.Background(), recordID, errors.New("boom"), 2)
		assert.Nil(t, err)

		batch, err := env.DB.FetchBatch(context.Background(), "email-received", "test-instance", 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(batch), "a record that reached maxAttempts must be quarantined, excluded from future batches")
	})
}

func TestInboxRepositoryClaimedBy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	env := utils.NewIntegrationTestEnvironment(t)
	_, err := env.DB.Save(context.Background(), "email-received", 0, 1, []byte("payload-1"))
	assert.Nil(t, err)

	records, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(records), "instance-a claims the only pending row")

	t.Run("the same instance immediately reclaims its own stuck row, no TTL wait", func(t *testing.T) {
		batch, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(batch), "instance-a's own processing row is reclaimable immediately, despite claimTTL not having elapsed")
	})

	t.Run("a different instance must still wait out claimTTL", func(t *testing.T) {
		batch, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-b", 10, 30*time.Second)
		assert.Nil(t, err, "instance-b sees a row claimed by instance-a, still within its TTL")
		assert.Equal(t, 0, len(batch), "instance-b must not reclaim another instance's row before its TTL expires")

		expired, err := env.DB.FetchBatch(context.Background(), "email-received", "instance-b", 10, 0*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(expired), "a claimTTL of 0 means instance-b can now reclaim instance-a's expired claim")
	})
}
