package outbox_test

import (
	"context"
	"errors"
	libsoutbox "planeo/libs/outbox"
	"planeo/services/email/internal/domain/outbox/mocks"
	"planeo/services/email/internal/infra/outbox"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEmailReceivedProducer(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("processes every fetched record, even if one fails", func(t *testing.T) {
		mockService := mocks.NewMockService(t)

		records := []libsoutbox.Record{
			{ID: 1, Topic: "email-received"},
			{ID: 2, Topic: "email-received"},
		}
		mockService.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return(records, nil)
		mockService.EXPECT().ProcessRecord(context.Background(), records[0], 5).Return(errors.New("boom"))
		mockService.EXPECT().ProcessRecord(context.Background(), records[1], 5).Return(nil)

		producer := outbox.NewEmailReceivedProducer(mockService, "email-received", "instance-a", 10, 5, 30*time.Second)

		err := producer.PollOnce(context.Background())

		assert.Nil(t, err)
	})

	t.Run("propagates a FetchBatch error without calling ProcessRecord", func(t *testing.T) {
		mockService := mocks.NewMockService(t)

		fetchErr := errors.New("fetch failed")
		mockService.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return(nil, fetchErr)

		producer := outbox.NewEmailReceivedProducer(mockService, "email-received", "instance-a", 10, 5, 30*time.Second)

		err := producer.PollOnce(context.Background())

		assert.Equal(t, fetchErr, err)
	})
}
