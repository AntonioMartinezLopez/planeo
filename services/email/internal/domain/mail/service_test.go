package mail_test

import (
	"context"
	. "planeo/services/email/internal/domain/mail"
	"planeo/services/email/internal/domain/mail/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMailService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	raws := []RawFetchedMail{
		{
			MessageID:      "abc123",
			SettingID:      1,
			OrganizationID: 1,
			Subject:        "Test",
			Sender:         "sender@example.com",
			Body:           "body",
			Date:           time.Now(),
			UID:            42,
		},
	}

	t.Run("SaveFetchedMails", func(t *testing.T) {
		t.Run("creates the mail and outbox event when the mail is newly inserted", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRepository.EXPECT().CreateMail(context.Background(), mock.AnythingOfType("NewMail")).Return(1, true, nil)
			mockRepository.EXPECT().CreateOutboxEvent(context.Background(), 1, mock.AnythingOfType("OutboxEvent")).Return(nil)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), raws)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.True(t, results[0].Inserted)
			assert.Equal(t, uint32(42), results[0].UID)
		})

		t.Run("does not create an outbox event when the mail already exists", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRepository.EXPECT().CreateMail(context.Background(), mock.AnythingOfType("NewMail")).Return(0, false, nil)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), raws)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(results))
			assert.False(t, results[0].Inserted)
			// CreateOutboxEvent has no .EXPECT() set above; mockery's generated
			// mock fails the test via its registered t.Cleanup assertion if an
			// unexpected call happens, which is what proves it was never called.
		})

		t.Run("returns error when CreateMail fails", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRepository.EXPECT().CreateMail(context.Background(), mock.AnythingOfType("NewMail")).Return(0, false, assert.AnError)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), raws)
			assert.Error(t, err)
			assert.Nil(t, results)
		})

		t.Run("returns nil without calling repository when raws is empty", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mailService := NewService(mockRepository)

			results, err := mailService.SaveFetchedMails(context.Background(), []RawFetchedMail{})
			assert.Nil(t, err)
			assert.Nil(t, results)
		})
	})
}
