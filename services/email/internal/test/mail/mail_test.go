package mail_test

import (
	"context"
	"planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMailRepository(t *testing.T) {
	env := utils.NewIntegrationTestEnvironment(t)

	newMail := mail.NewMail{
		MessageID:      "duplicate-test-1",
		SettingID:      1,
		OrganizationID: 1,
		Subject:        "Test Subject",
		Sender:         "sender@example.com",
		Body:           "Test body",
		Date:           time.Now(),
	}
	event := mail.OutboxEvent{
		Topic:   "email-received",
		Key:     []byte("1"),
		Payload: []byte(`{"subject":"Test Subject"}`),
	}

	saveOnce := func(t *testing.T) (mailID int, inserted bool) {
		t.Helper()
		err := env.DB.WithTransaction(context.Background(), func(ctx context.Context) error {
			var err error
			mailID, inserted, err = env.DB.CreateMail(ctx, newMail)
			if err != nil {
				return err
			}
			if inserted {
				return env.DB.CreateOutboxEvent(ctx, mailID, event)
			}
			return nil
		})
		assert.Nil(t, err)
		return mailID, inserted
	}

	t.Run("CreateMail and CreateOutboxEvent within a transaction", func(t *testing.T) {
		t.Run("inserts a new mail and outbox event", func(t *testing.T) {
			mailID, inserted := saveOnce(t)
			assert.True(t, inserted)
			assert.NotZero(t, mailID)
		})

		t.Run("is idempotent on a duplicate setting_id+message_id", func(t *testing.T) {
			_, inserted := saveOnce(t)
			assert.False(t, inserted, "a conflicting mail must not create a second row, and must be reported as not-newly-inserted")
		})
	})
}
