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

	t.Run("SaveFetchedMails", func(t *testing.T) {
		t.Run("inserts a new mail and outbox event", func(t *testing.T) {
			fetched := []mail.FetchedMail{{Mail: newMail, Event: event, UID: 1}}

			results, err := env.DB.SaveFetchedMails(context.Background(), fetched)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.True(t, results[0].Inserted)
			assert.Equal(t, uint32(1), results[0].UID)
		})

		t.Run("is idempotent on a duplicate setting_id+message_id", func(t *testing.T) {
			fetched := []mail.FetchedMail{{Mail: newMail, Event: event, UID: 2}}

			results, err := env.DB.SaveFetchedMails(context.Background(), fetched)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.False(t, results[0].Inserted, "a conflicting mail must not create a second row, and must be reported as not-newly-inserted")
			assert.Equal(t, uint32(2), results[0].UID, "the UID from THIS fetch must still be returned so the caller can mark it seen")
		})
	})
}
