package inbox_test

import (
	"context"
	"errors"
	libsinbox "planeo/libs/inbox"
	"planeo/libs/logger"
	"planeo/services/core/internal/domain/inbox/mocks"
	"planeo/services/core/internal/infra/inbox"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEmailReceivedConsumer(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	ctx := logger.WithContext(context.Background(), logger.New("test"))

	t.Run("processes every fetched record, even if one fails", func(t *testing.T) {
		mockService := mocks.NewMockService(t)

		records := []libsinbox.Record{
			{ID: 1, Topic: "email-received"},
			{ID: 2, Topic: "email-received"},
		}
		mockService.EXPECT().
			FetchBatch(ctx, "email-received", "instance-a", 10, 30*time.Second).
			Return(records, nil)
		mockService.EXPECT().ProcessEmailReceived(ctx, records[0], 5).Return(errors.New("boom"))
		mockService.EXPECT().ProcessEmailReceived(ctx, records[1], 5).Return(nil)

		consumer := inbox.NewEmailReceivedConsumer(mockService, "email-received", "instance-a", 10, 5, 30*time.Second)

		err := consumer.PollOnce(ctx)

		assert.Nil(t, err)
	})

	t.Run("propagates a FetchBatch error without calling ProcessEmailReceived", func(t *testing.T) {
		mockService := mocks.NewMockService(t)

		fetchErr := errors.New("fetch failed")
		mockService.EXPECT().
			FetchBatch(ctx, "email-received", "instance-a", 10, 30*time.Second).
			Return(nil, fetchErr)

		consumer := inbox.NewEmailReceivedConsumer(mockService, "email-received", "instance-a", 10, 5, 30*time.Second)

		err := consumer.PollOnce(ctx)

		assert.Equal(t, fetchErr, err)
	})
}
