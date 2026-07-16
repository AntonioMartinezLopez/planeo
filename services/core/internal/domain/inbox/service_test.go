package inbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"planeo/libs/events/contracts"
	libsinbox "planeo/libs/inbox"
	"planeo/services/core/internal/domain/category"
	categorymocks "planeo/services/core/internal/domain/category/mocks"
	. "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/domain/inbox/mocks"
	"planeo/services/core/internal/domain/request"
	requestmocks "planeo/services/core/internal/domain/request/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// wantRaw is the "raw" string ProcessEmailReceived builds from validPayload's
// fixed fields - hardcoded here (rather than recomputed) so a test failure
// clearly shows a divergence from the service's own formatting.
const wantRaw = "Subject: Need a permit\nFrom: jane@example.com\nDate: Thu, 01 Jan 2026 00:00:00 UTC\nMessage-ID: msg-1\nBody: Please help"

func categoryIDPtr(id int) *int { return &id }

func validPayload(t *testing.T) []byte {
	t.Helper()
	payload := contracts.EmailCreatedPayload{
		Subject:        "Need a permit",
		Body:           "Please help",
		From:           "jane@example.com",
		Date:           time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MessageID:      "msg-1",
		OrganizationId: 1,
	}
	raw, err := json.Marshal(payload)
	assert.Nil(t, err)
	return raw
}

func TestService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	t.Run("FetchBatch delegates to the repository", func(t *testing.T) {
		mockRepository := mocks.NewMockRepository(t)
		record := libsinbox.Record{ID: 1, Topic: "email-received", Payload: []byte("{}")}
		mockRepository.EXPECT().
			FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second).
			Return([]libsinbox.Record{record}, nil)

		service := NewService(mockRepository, requestmocks.NewMockService(t), categorymocks.NewMockService(t), mocks.NewMockLLMClient(t))

		records, err := service.FetchBatch(context.Background(), "email-received", "instance-a", 10, 30*time.Second)

		assert.Nil(t, err)
		assert.Equal(t, []libsinbox.Record{record}, records)
	})

	t.Run("ProcessEmailReceived", func(t *testing.T) {
		t.Run("upserts the request, then marks the record processed", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRequestService := requestmocks.NewMockService(t)
			mockCategoryService := categorymocks.NewMockService(t)
			mockLLMClient := mocks.NewMockLLMClient(t)

			rec := libsinbox.Record{ID: 1, Topic: "email-received", Payload: validPayload(t)}
			categories := []category.Category{{Id: 5, Label: "Permits"}}

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(categories, nil)
			mockLLMClient.EXPECT().
				ExtractRequestFields(context.Background(), mock.AnythingOfType("string")).
				Return(ExtractorOutput{Name: "Jane", Address: "Main St", Phone: "123"}, nil)
			mockLLMClient.EXPECT().
				ClassifyRequest(context.Background(), RequestData{Subject: "Need a permit", Text: "Please help"}, categories).
				Return(5, nil)

			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRequestService.EXPECT().
				UpsertRequest(context.Background(), request.Request{
					Text:           "Please help",
					Subject:        "Need a permit",
					Email:          "jane@example.com",
					Name:           "Jane",
					Address:        "Main St",
					Telephone:      "123",
					Raw:            wantRaw,
					ReferenceId:    "msg-1",
					CategoryId:     categoryIDPtr(5),
					OrganizationId: 1,
				}).
				Return(42, nil)
			mockRepository.EXPECT().MarkProcessed(context.Background(), int64(1)).Return(nil)

			service := NewService(mockRepository, mockRequestService, mockCategoryService, mockLLMClient)

			err := service.ProcessEmailReceived(context.Background(), rec, 5)

			assert.Nil(t, err)
		})

		t.Run("marks the record failed when the payload is not valid JSON", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			rec := libsinbox.Record{ID: 2, Topic: "email-received", Payload: []byte("not-json")}

			mockRepository.EXPECT().MarkFailed(context.Background(), int64(2), mock.Anything, 5).Return(nil)

			service := NewService(mockRepository, requestmocks.NewMockService(t), categorymocks.NewMockService(t), mocks.NewMockLLMClient(t))

			err := service.ProcessEmailReceived(context.Background(), rec, 5)

			assert.Nil(t, err)
		})

		t.Run("marks the record failed when fetching categories errors", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockCategoryService := categorymocks.NewMockService(t)
			rec := libsinbox.Record{ID: 3, Topic: "email-received", Payload: validPayload(t)}
			categoryErr := errors.New("category lookup failed")

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(nil, categoryErr)
			mockRepository.EXPECT().MarkFailed(context.Background(), int64(3), categoryErr, 5).Return(nil)

			service := NewService(mockRepository, requestmocks.NewMockService(t), mockCategoryService, mocks.NewMockLLMClient(t))

			err := service.ProcessEmailReceived(context.Background(), rec, 5)

			assert.Nil(t, err)
		})

		t.Run("proceeds to classification and request creation when field extraction fails", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRequestService := requestmocks.NewMockService(t)
			mockCategoryService := categorymocks.NewMockService(t)
			mockLLMClient := mocks.NewMockLLMClient(t)

			rec := libsinbox.Record{ID: 4, Topic: "email-received", Payload: validPayload(t)}
			categories := []category.Category{{Id: 5, Label: "Permits"}}

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(categories, nil)
			mockLLMClient.EXPECT().
				ExtractRequestFields(context.Background(), mock.AnythingOfType("string")).
				Return(ExtractorOutput{}, errors.New("extraction unavailable"))
			mockLLMClient.EXPECT().
				ClassifyRequest(context.Background(), RequestData{Subject: "Need a permit", Text: "Please help"}, categories).
				Return(5, nil)

			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRequestService.EXPECT().
				UpsertRequest(context.Background(), mock.AnythingOfType("request.Request")).
				Return(42, nil)
			mockRepository.EXPECT().MarkProcessed(context.Background(), int64(4)).Return(nil)

			service := NewService(mockRepository, mockRequestService, mockCategoryService, mockLLMClient)

			err := service.ProcessEmailReceived(context.Background(), rec, 5)

			assert.Nil(t, err)
		})

		t.Run("marks the record failed when classification errors", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockCategoryService := categorymocks.NewMockService(t)
			mockLLMClient := mocks.NewMockLLMClient(t)

			rec := libsinbox.Record{ID: 5, Topic: "email-received", Payload: validPayload(t)}
			categories := []category.Category{{Id: 5, Label: "Permits"}}
			classifyErr := errors.New("classification failed")

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(categories, nil)
			mockLLMClient.EXPECT().
				ExtractRequestFields(context.Background(), mock.AnythingOfType("string")).
				Return(ExtractorOutput{}, nil)
			mockLLMClient.EXPECT().
				ClassifyRequest(context.Background(), RequestData{Subject: "Need a permit", Text: "Please help"}, categories).
				Return(0, classifyErr)
			mockRepository.EXPECT().MarkFailed(context.Background(), int64(5), classifyErr, 5).Return(nil)

			service := NewService(mockRepository, requestmocks.NewMockService(t), mockCategoryService, mockLLMClient)

			err := service.ProcessEmailReceived(context.Background(), rec, 5)

			assert.Nil(t, err)
		})

		t.Run("marks the record failed outside the transaction when the write transaction errors", func(t *testing.T) {
			mockRepository := mocks.NewMockRepository(t)
			mockRequestService := requestmocks.NewMockService(t)
			mockCategoryService := categorymocks.NewMockService(t)
			mockLLMClient := mocks.NewMockLLMClient(t)

			rec := libsinbox.Record{ID: 6, Topic: "email-received", Payload: validPayload(t)}
			categories := []category.Category{{Id: 5, Label: "Permits"}}
			txErr := errors.New("write failed")

			mockCategoryService.EXPECT().GetCategories(context.Background(), 1).Return(categories, nil)
			mockLLMClient.EXPECT().
				ExtractRequestFields(context.Background(), mock.AnythingOfType("string")).
				Return(ExtractorOutput{}, nil)
			mockLLMClient.EXPECT().
				ClassifyRequest(context.Background(), RequestData{Subject: "Need a permit", Text: "Please help"}, categories).
				Return(5, nil)

			mockRepository.EXPECT().
				WithTransaction(context.Background(), mock.Anything).
				RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
					return fn(ctx)
				})
			mockRequestService.EXPECT().
				UpsertRequest(context.Background(), mock.AnythingOfType("request.Request")).
				Return(0, txErr)
			mockRepository.EXPECT().MarkFailed(context.Background(), int64(6), txErr, 5).Return(nil)

			service := NewService(mockRepository, mockRequestService, mockCategoryService, mockLLMClient)

			err := service.ProcessEmailReceived(context.Background(), rec, 5)

			assert.Nil(t, err)
		})
	})
}
