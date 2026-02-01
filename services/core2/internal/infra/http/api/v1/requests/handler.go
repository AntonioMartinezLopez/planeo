package requests

import (
	"context"
	"planeo/services/core2/internal/domain/request"
	"planeo/services/core2/internal/infra/http/server"
)

type RequestController struct {
	requestService request.Service
}

func NewRequestController(requestService request.Service) *RequestController {
	return &RequestController{
		requestService: requestService,
	}
}

func (r *RequestController) GetRequests(ctx context.Context, input *GetRequestsInput) (*GetRequestsOutput, error) {
	result, err := r.requestService.GetRequests(ctx, input.OrganizationId, input.Cursor, input.PageSize, input.GetClosed, input.SelectedCategories)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	resp := &GetRequestsOutput{}
	resp.Body.Requests = result
	resp.Body.Limit = input.PageSize

	if len(result) > 0 {
		resp.Body.NextCursor = result[len(result)-1].Id
	}

	return resp, nil
}

func (r *RequestController) CreateRequest(ctx context.Context, input *CreateRequestInput) (*CreateRequestOutput, error) {
	newRequest := request.NewRequest{
		Text:           input.Body.Text,
		Name:           input.Body.Name,
		Email:          input.Body.Email,
		Address:        input.Body.Address,
		Telephone:      input.Body.Telephone,
		Closed:         input.Body.Closed,
		OrganizationId: input.OrganizationId,
		CategoryId:     input.Body.CategoryId,
	}
	result, err := r.requestService.CreateRequest(ctx, newRequest)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}
	resp := &CreateRequestOutput{}
	resp.Body.Id = result
	return resp, nil
}

func (r *RequestController) UpdateRequest(ctx context.Context, input *UpdateRequestInput) (*struct{}, error) {
	request := request.UpdateRequest{
		Text:           input.Body.Text,
		Name:           input.Body.Name,
		Email:          input.Body.Email,
		Address:        input.Body.Address,
		Telephone:      input.Body.Telephone,
		Closed:         input.Body.Closed,
		CategoryId:     input.Body.CategoryId,
		OrganizationId: input.OrganizationId,
		Id:             input.RequestId,
	}
	err := r.requestService.UpdateRequest(ctx, request)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	return nil, nil
}

func (r *RequestController) DeleteRequest(ctx context.Context, input *DeleteRequestInput) (*struct{}, error) {
	err := r.requestService.DeleteRequest(ctx, input.OrganizationId, input.RequestId)

	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	return nil, nil
}
