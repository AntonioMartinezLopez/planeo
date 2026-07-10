package mail_test

import (
	"context"
	. "planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/domain/mail/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMailService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	fetched := []FetchedMail{
		{
			Mail: NewMail{
				MessageID:      "abc123",
				SettingID:      1,
				OrganizationID: 1,
				Subject:        "Test",
				Sender:         "sender@example.com",
				Body:           "body",
				Date:           time.Now(),
			},
			Event: OutboxEvent{
				Topic:   "email-received",
				Key:     []byte("1"),
				Payload: []byte(`{"subject":"Test"}`),
			},
			UID: 42,
		},
	}

	t.Run("SaveFetchedMails", func(t *testing.T) {
		t.Run("returns results when mails are saved successfully", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().SaveFetchedMails(context.Background(), fetched).Return([]SaveResult{{UID: 42, Inserted: true}}, nil)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), fetched)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.True(t, results[0].Inserted)
		})

		t.Run("returns error when saving fails", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().SaveFetchedMails(context.Background(), fetched).Return(nil, assert.AnError)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), fetched)
			assert.Error(t, err)
			assert.Nil(t, results)
		})

		t.Run("returns nil without calling repository when mails is empty", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), []FetchedMail{})
			assert.Nil(t, err)
			assert.Nil(t, results)
		})
	})
}
