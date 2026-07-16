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
		mockProducer := mocks.NewMockProducer(t)

		record := libsoutbox.Record{ID: 1, Topic: "email-received", Key: []byte("k"), Payload: []byte("v")}
		mockRepository.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return([]libsoutbox.Record{record}, nil)

		service := NewService(mockRepository, mockProducer)

		records, err := service.FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second)

		assert.Nil(t, err)
		assert.Equal(t, []libsoutbox.Record{record}, records)
	})

	t.Run("ProcessRecord produces a record and marks it processed", func(t *testing.T) {
		mockRepository := mocks.NewMockRepository(t)
		mockProducer := mocks.NewMockProducer(t)

		record := libsoutbox.Record{ID: 1, Topic: "email-received", Key: []byte("k"), Payload: []byte("v")}
		mockProducer.EXPECT().ProduceSync(context.Background(), "email-received", []byte("k"), []byte("v")).Return(nil)
		mockRepository.EXPECT().MarkProcessed(context.Background(), int64(1)).Return(nil)

		service := NewService(mockRepository, mockProducer)

		err := service.ProcessRecord(context.Background(), record, 5)

		assert.Nil(t, err)
	})

	t.Run("ProcessRecord marks a record failed when producing errors", func(t *testing.T) {
		mockRepository := mocks.NewMockRepository(t)
		mockProducer := mocks.NewMockProducer(t)

		record := libsoutbox.Record{ID: 2, Topic: "broken-topic", Key: nil, Payload: []byte("v")}
		sendErr := errors.New("simulated produce failure")
		mockProducer.EXPECT().ProduceSync(context.Background(), "broken-topic", ([]byte)(nil), []byte("v")).Return(sendErr)
		mockRepository.EXPECT().MarkFailed(context.Background(), int64(2), sendErr, 2).Return(nil)

		service := NewService(mockRepository, mockProducer)

		err := service.ProcessRecord(context.Background(), record, 2)

		assert.Nil(t, err)
	})
}
