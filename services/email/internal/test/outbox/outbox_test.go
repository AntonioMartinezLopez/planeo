package outbox_test

import (
	"context"
	"errors"
	"planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func seedOutboxEvent(t *testing.T, env *utils.IntegrationTestEnvironment, messageID string) {
	t.Helper()

	newMail := mail.NewMail{
		MessageID:      messageID,
		SettingID:      1,
		OrganizationID: 1,
		Subject:        "Subject",
		Sender:         "sender@example.com",
		Body:           "Body",
		Date:           time.Now(),
	}
	event := mail.OutboxEvent{
		Topic:   "email-received",
		Key:     []byte("1"),
		Payload: []byte(`{"subject":"Subject"}`),
	}

	err := env.DB.WithTransaction(context.Background(), func(ctx context.Context) error {
		mailID, inserted, err := env.DB.CreateMail(ctx, newMail)
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		return env.DB.CreateOutboxEvent(ctx, mailID, event)
	})
	assert.Nil(t, err)
}

func TestOutboxRepository(t *testing.T) {
	env := utils.NewIntegrationTestEnvironment(t)
	seedOutboxEvent(t, env, "outbox-test-1")

	t.Run("FetchBatch", func(t *testing.T) {
		t.Run("claims a pending record", func(t *testing.T) {
			records, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(records))
			assert.Equal(t, "email-received", records[0].Topic)
		})

		t.Run("does not reclaim a record still within its claim TTL", func(t *testing.T) {
			records, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
			assert.Nil(t, err)
			assert.Equal(t, 0, len(records), "the record claimed in the previous poll is still within its TTL")
		})

		t.Run("reclaims a record whose claim has expired", func(t *testing.T) {
			records, err := env.DB.FetchBatch(context.Background(), 10, 0*time.Second)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(records), "a claimTTL of 0 means any processing record is immediately reclaimable")
		})
	})

	t.Run("MarkProcessed", func(t *testing.T) {
		t.Run("marks the record sent and excludes it from future batches", func(t *testing.T) {
			err := env.DB.MarkProcessed(context.Background(), 1)
			assert.Nil(t, err)

			records, err := env.DB.FetchBatch(context.Background(), 10, 0*time.Second)
			assert.Nil(t, err)
			assert.Equal(t, 0, len(records))
		})
	})
}

func TestOutboxRepositoryMarkFailed(t *testing.T) {
	env := utils.NewIntegrationTestEnvironment(t)
	seedOutboxEvent(t, env, "outbox-test-2")

	records, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(records))
	recordID := records[0].ID

	t.Run("resets to pending when attempts is still below maxAttempts", func(t *testing.T) {
		err := env.DB.MarkFailed(context.Background(), recordID, errors.New("boom"), 3)
		assert.Nil(t, err)

		batch, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(batch), "a not-yet-exhausted failure must be reset to pending, not left claimed")
	})

	t.Run("quarantines the record once maxAttempts is reached", func(t *testing.T) {
		err := env.DB.MarkFailed(context.Background(), recordID, errors.New("boom"), 2)
		assert.Nil(t, err)

		batch, err := env.DB.FetchBatch(context.Background(), 10, 30*time.Second)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(batch), "a record that reached maxAttempts must be quarantined, excluded from future batches")
	})
}
