package outbox_test

import (
	"context"
	"errors"
	libsoutbox "planeo/libs/outbox"
	. "planeo/services/email/internal/domain/outbox"
	"planeo/services/email/internal/domain/outbox/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("FetchBatch delegates to the repository", func(t *testing.T) {
		mockRepository := mocks.NewMockRepository(t)

		record := libsoutbox.Record{ID: 1, Topic: "email-received", Key: []byte("k"), Payload: []byte("v")}
		mockRepository.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return([]libsoutbox.Record{record}, nil)

		service := NewService(mockRepository)

		records, err := service.FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second)

		assert.Nil(t, err)
		assert.Equal(t, []libsoutbox.Record{record}, records)
	})

	t.Run("MarkProcessed delegates to the repository", func(t *testing.T) {
		mockRepository := mocks.NewMockRepository(t)
		mockRepository.EXPECT().MarkProcessed(context.Background(), int64(1)).Return(nil)

		service := NewService(mockRepository)

		err := service.MarkProcessed(context.Background(), 1)

		assert.Nil(t, err)
	})

	t.Run("MarkFailed delegates to the repository", func(t *testing.T) {
		mockRepository := mocks.NewMockRepository(t)
		sendErr := errors.New("simulated produce failure")
		mockRepository.EXPECT().MarkFailed(context.Background(), int64(2), sendErr, 2).Return(nil)

		service := NewService(mockRepository)

		err := service.MarkFailed(context.Background(), 2, sendErr, 2)

		assert.Nil(t, err)
	})
}
