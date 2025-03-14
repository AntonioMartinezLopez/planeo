package request

import (
	"context"
	"planeo/services/core/internal/resources/request/mocks"
	"planeo/services/core/internal/resources/request/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRequestService(t *testing.T) {

	if !testing.Short() {
		t.Skip()
	}

	testOrganizationId := 1
	requestCreateInput := models.NewRequest{
		Text:           "Some request text",
		Name:           "John Doe",
		Email:          "john.doe@test.com",
		Address:        "123 Main St",
		Telephone:      "123-456-7890",
		Closed:         false,
		CategoryId:     1,
		OrganizationId: testOrganizationId,
	}
	requestUpdateInput := models.UpdateRequest{
		Text:           "Some updated request text",
		Name:           "Jane Doe",
		Email:          "john.doe@test.com",
		Address:        "123 Main St",
		Telephone:      "123-456-7890",
		Closed:         false,
		CategoryId:     1,
		OrganizationId: testOrganizationId,
		Id:             1,
	}
	request := models.Request{
		Text:           requestCreateInput.Text,
		Name:           requestCreateInput.Name,
		Email:          requestCreateInput.Email,
		Address:        requestCreateInput.Address,
		Telephone:      requestCreateInput.Telephone,
		Closed:         requestCreateInput.Closed,
		OrganizationId: testOrganizationId,
		Id:             1,
		CategoryId:     nil,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	t.Run("CreateRequest", func(t *testing.T) {

		t.Run("returns nil when request is created successfully", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepositoryInterface(t)
			mockRequestRepository.EXPECT().CreateRequest(context.Background(), requestCreateInput).Return(nil)
			requestService := NewRequestService(mockRequestRepository)

			err := requestService.CreateRequest(context.Background(), requestCreateInput)
			assert.Nil(t, err)
		})

		t.Run("returns error when request creation fails", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepositoryInterface(t)
			mockRequestRepository.EXPECT().CreateRequest(context.Background(), requestCreateInput).Return(assert.AnError)
			requestService := NewRequestService(mockRequestRepository)

			err := requestService.CreateRequest(context.Background(), requestCreateInput)
			assert.Error(t, err)
		})
	})

	t.Run("UpdateRequest", func(t *testing.T) {

		t.Run("returns nil when request is updated successfully", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepositoryInterface(t)
			mockRequestRepository.EXPECT().UpdateRequest(context.Background(), requestUpdateInput).Return(nil)
			requestService := NewRequestService(mockRequestRepository)

			err := requestService.UpdateRequest(context.Background(), requestUpdateInput)
			assert.Nil(t, err)
		})

		t.Run("returns error when request update fails", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepositoryInterface(t)
			mockRequestRepository.EXPECT().UpdateRequest(context.Background(), requestUpdateInput).Return(assert.AnError)
			requestService := NewRequestService(mockRequestRepository)

			err := requestService.UpdateRequest(context.Background(), requestUpdateInput)
			assert.Error(t, err)
		})
	})

	t.Run("DeleteRequest", func(t *testing.T) {

		t.Run("returns nil when request is deleted successfully", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepositoryInterface(t)
			mockRequestRepository.EXPECT().DeleteRequest(context.Background(), testOrganizationId, request.Id).Return(nil)
			requestService := NewRequestService(mockRequestRepository)

			err := requestService.DeleteRequest(context.Background(), testOrganizationId, request.Id)
			assert.Nil(t, err)
		})

		t.Run("returns error when request deletion fails", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepositoryInterface(t)
			mockRequestRepository.EXPECT().DeleteRequest(context.Background(), testOrganizationId, request.Id).Return(assert.AnError)
			requestService := NewRequestService(mockRequestRepository)

			err := requestService.DeleteRequest(context.Background(), testOrganizationId, request.Id)
			assert.Error(t, err)
		})
	})

	t.Run("GetRequests", func(t *testing.T) {

		t.Run("returns requests when requests are fetched successfully", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepositoryInterface(t)
			mockRequestRepository.EXPECT().GetRequests(context.Background(), testOrganizationId, 0, 10, false).Return([]models.Request{request}, nil)
			requestService := NewRequestService(mockRequestRepository)

			requests, err := requestService.GetRequests(context.Background(), testOrganizationId, 0, 10, false)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(requests))
		})

		t.Run("returns error when requests fetch fails", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepositoryInterface(t)
			mockRequestRepository.EXPECT().GetRequests(context.Background(), testOrganizationId, 0, 10, false).Return(nil, assert.AnError)
			requestService := NewRequestService(mockRequestRepository)

			requests, err := requestService.GetRequests(context.Background(), testOrganizationId, 0, 10, false)
			assert.Error(t, err)
			assert.Nil(t, requests)
		})
	})

}
