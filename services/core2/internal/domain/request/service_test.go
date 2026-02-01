package request_test

import (
	"context"
	. "planeo/services/core2/internal/domain/request"
	"planeo/services/core2/internal/domain/request/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRequestService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	testOrganizationId := 1
	requestCreateInput := NewRequest{
		Subject:        "Some request subject",
		Text:           "Some request text",
		Name:           "John Doe",
		Email:          "john.doe@test.com",
		Address:        "123 Main St",
		Telephone:      "123-456-7890",
		Closed:         false,
		CategoryId:     1,
		OrganizationId: testOrganizationId,
	}
	requestUpdateInput := UpdateRequest{
		Subject:        "Some request subject",
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
	request := Request{
		Subject:        requestCreateInput.Subject,
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
			mockRequestRepository := mocks.NewMockRequestRepository(t)
			mockRequestRepository.EXPECT().CreateRequest(context.Background(), requestCreateInput).Return(1, nil)
			requestService := NewService(mockRequestRepository)

			id, err := requestService.CreateRequest(context.Background(), requestCreateInput)
			assert.Nil(t, err)
			assert.Equal(t, 1, id)
		})

		t.Run("returns error when request creation fails", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepository(t)
			mockRequestRepository.EXPECT().CreateRequest(context.Background(), requestCreateInput).Return(0, assert.AnError)
			requestService := NewService(mockRequestRepository)

			id, err := requestService.CreateRequest(context.Background(), requestCreateInput)
			assert.Error(t, err)
			assert.Equal(t, 0, id)
		})
	})

	t.Run("UpdateRequest", func(t *testing.T) {
		t.Run("returns nil when request is updated successfully", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepository(t)
			mockRequestRepository.EXPECT().UpdateRequest(context.Background(), requestUpdateInput).Return(nil)
			requestService := NewService(mockRequestRepository)

			err := requestService.UpdateRequest(context.Background(), requestUpdateInput)
			assert.Nil(t, err)
		})

		t.Run("returns error when request update fails", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepository(t)
			mockRequestRepository.EXPECT().UpdateRequest(context.Background(), requestUpdateInput).Return(assert.AnError)
			requestService := NewService(mockRequestRepository)

			err := requestService.UpdateRequest(context.Background(), requestUpdateInput)
			assert.Error(t, err)
		})
	})

	t.Run("DeleteRequest", func(t *testing.T) {
		t.Run("returns nil when request is deleted successfully", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepository(t)
			mockRequestRepository.EXPECT().DeleteRequest(context.Background(), testOrganizationId, request.Id).Return(nil)
			requestService := NewService(mockRequestRepository)

			err := requestService.DeleteRequest(context.Background(), testOrganizationId, request.Id)
			assert.Nil(t, err)
		})

		t.Run("returns error when request deletion fails", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepository(t)
			mockRequestRepository.EXPECT().DeleteRequest(context.Background(), testOrganizationId, request.Id).Return(assert.AnError)
			requestService := NewService(mockRequestRepository)

			err := requestService.DeleteRequest(context.Background(), testOrganizationId, request.Id)
			assert.Error(t, err)
		})
	})

	t.Run("GetRequests", func(t *testing.T) {
		t.Run("returns requests when requests are fetched successfully", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepository(t)
			mockRequestRepository.EXPECT().GetRequests(context.Background(), testOrganizationId, 0, 10, false, []int{}).Return([]Request{request}, nil)
			requestService := NewService(mockRequestRepository)

			requests, err := requestService.GetRequests(context.Background(), testOrganizationId, 0, 10, false, []int{})
			assert.Nil(t, err)
			assert.Equal(t, 1, len(requests))
		})

		t.Run("returns error when requests fetch fails", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepository(t)
			mockRequestRepository.EXPECT().GetRequests(context.Background(), testOrganizationId, 0, 10, false, []int{}).Return(nil, assert.AnError)
			requestService := NewService(mockRequestRepository)

			requests, err := requestService.GetRequests(context.Background(), testOrganizationId, 0, 10, false, []int{})
			assert.Error(t, err)
			assert.Nil(t, requests)
		})

		t.Run("returns filtered requests when selectedCategories is provided", func(t *testing.T) {
			mockRequestRepository := mocks.NewMockRequestRepository(t)
			selectedCategories := []int{1, 2}
			mockRequestRepository.EXPECT().GetRequests(context.Background(), testOrganizationId, 0, 10, false, selectedCategories).Return([]Request{request}, nil)
			requestService := NewService(mockRequestRepository)

			requests, err := requestService.GetRequests(context.Background(), testOrganizationId, 0, 10, false, selectedCategories)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(requests))
		})
	})

}
