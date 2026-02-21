package requests

import (
	"context"
	"net/http"
	humaUtils "planeo/libs/huma_utils"
	"planeo/libs/middlewares"
	"planeo/services/core/internal/domain/request"

	. "planeo/services/core/internal/infra/rest/api"

	"github.com/danielgtaylor/huma/v2"
)

type RequestHandler struct {
	requestService request.Service
}

func NewRequestHandler(requestService request.Service) *RequestHandler {
	return &RequestHandler{
		requestService: requestService,
	}
}

func (r *RequestHandler) GetRequests(ctx context.Context, input *GetRequestsInput) (*GetRequestsOutput, error) {
	result, err := r.requestService.GetRequests(ctx, input.OrganizationId, input.Cursor, input.PageSize, input.GetClosed, input.SelectedCategories)

	if err != nil {
		return nil, NewHTTPError(err)
	}

	resp := &GetRequestsOutput{}
	resp.Body.Requests = result
	resp.Body.Limit = input.PageSize

	if len(result) > 0 {
		resp.Body.NextCursor = result[len(result)-1].Id
	}

	return resp, nil
}

func (r *RequestHandler) CreateRequest(ctx context.Context, input *CreateRequestInput) (*CreateRequestOutput, error) {
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
		return nil, NewHTTPError(err)
	}
	resp := &CreateRequestOutput{}
	resp.Body.Id = result
	return resp, nil
}

func (r *RequestHandler) UpdateRequest(ctx context.Context, input *UpdateRequestInput) (*struct{}, error) {
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
		return nil, NewHTTPError(err)
	}

	return nil, nil
}

func (r *RequestHandler) DeleteRequest(ctx context.Context, input *DeleteRequestInput) (*struct{}, error) {
	err := r.requestService.DeleteRequest(ctx, input.OrganizationId, input.RequestId)

	if err != nil {
		return nil, NewHTTPError(err)
	}

	return nil, nil
}

func (r *RequestHandler) RegisterRoutes(api huma.API, permissions middlewares.PermissionMiddlewareConfig) {
	huma.Register(api, humaUtils.WithAuth(huma.Operation{
		OperationID: "get-requests",
		Method:      http.MethodGet,
		Path:        "/v1/organizations/{organizationId}/requests",
		Summary:     "Get Requests",
		Tags:        []string{"Requests"},
		Middlewares: huma.Middlewares{permissions.Apply("request", "read")},
	}), r.GetRequests)

	huma.Register(api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "create-request",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/v1/organizations/{organizationId}/requests",
		Summary:       "Create Request",
		Tags:          []string{"Requests"},
		Middlewares:   huma.Middlewares{permissions.Apply("request", "create")},
	}), r.CreateRequest)

	huma.Register(api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "update-request",
		Method:        http.MethodPut,
		DefaultStatus: http.StatusNoContent,
		Path:          "/v1/organizations/{organizationId}/requests/{requestId}",
		Summary:       "Update Request",
		Tags:          []string{"Requests"},
		Middlewares:   huma.Middlewares{permissions.Apply("request", "update")},
	}), r.UpdateRequest)

	huma.Register(api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "delete-request",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/v1/organizations/{organizationId}/requests/{requestId}",
		Summary:       "Delete Request",
		Tags:          []string{"Requests"},
		Middlewares:   huma.Middlewares{permissions.Apply("request", "delete")},
	}), r.DeleteRequest)
}
