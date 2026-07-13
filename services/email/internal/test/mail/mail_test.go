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

		t.Run("rolls back the mail row when CreateOutboxEvent fails", func(t *testing.T) {
			rollbackMail := mail.NewMail{
				MessageID:      "rollback-test-1",
				SettingID:      1,
				OrganizationID: 1,
				Subject:        "Rollback Test",
				Sender:         "sender@example.com",
				Body:           "Test body",
				Date:           time.Now(),
			}

			err := env.DB.WithTransaction(context.Background(), func(ctx context.Context) error {
				mailID, inserted, err := env.DB.CreateMail(ctx, rollbackMail)
				if err != nil {
					return err
				}
				assert.True(t, inserted)

				const nonExistentMailID = 999999999
				_ = mailID
				return env.DB.CreateOutboxEvent(ctx, nonExistentMailID, event)
			})
			assert.Error(t, err, "CreateOutboxEvent must fail on a foreign-key violation (mail_id references a non-existent mails row)")

			// If the failed transaction had NOT rolled back, this row would already
			// exist and CreateMail would report inserted == false (a duplicate on
			// setting_id+message_id). Getting inserted == true here proves the
			// earlier CreateMail was rolled back along with the failed CreateOutboxEvent.
			_, insertedAfterRollback, err := env.DB.CreateMail(context.Background(), rollbackMail)
			assert.Nil(t, err)
			assert.True(t, insertedAfterRollback, "the mail row from the failed transaction must not have survived — this insert should succeed as if for the first time")
		})
	})
}
