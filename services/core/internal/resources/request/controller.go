package request

import (
	"context"
	"net/http"
	humaUtils "planeo/libs/huma_utils"
	"planeo/libs/middlewares"
	"planeo/services/core/config"
	"planeo/services/core/internal/resources/request/dto"
	"planeo/services/core/internal/resources/request/models"

	"github.com/danielgtaylor/huma/v2"
)

type RequestController struct {
	api            huma.API
	requestService *RequestService
	config         *config.ApplicationConfiguration
}

func NewRequestController(api huma.API, config *config.ApplicationConfiguration, requestService *RequestService) *RequestController {
	return &RequestController{
		api:            api,
		requestService: requestService,
		config:         config,
	}
}

func (r *RequestController) InitializeRoutes() {
	permissions := middlewares.NewPermissionMiddlewareConfig(r.api, r.config.OauthIssuerUrl(), r.config.KcOauthClientID)
	huma.Register(r.api, humaUtils.WithAuth(huma.Operation{
		OperationID: "get-requests",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/requests",
		Summary:     "Get Requests",
		Tags:        []string{"Requests"},
		Middlewares: huma.Middlewares{permissions.Apply("request", "read")},
	}), func(ctx context.Context, input *dto.GetRequestsInput) (*dto.GetRequestsOutput, error) {

		result, err := r.requestService.GetRequests(ctx, input.OrganizationId, input.Cursor, input.PageSize, input.GetClosed)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		resp := &dto.GetRequestsOutput{}
		resp.Body.Requests = result
		resp.Body.Limit = input.PageSize

		if len(result) > 0 {
			resp.Body.NextCursor = result[len(result)-1].Id
		}

		return resp, nil
	})

	huma.Register(r.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "create-request",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/organizations/{organizationId}/requests",
		Summary:       "Create Request",
		Tags:          []string{"Requests"},
		Middlewares:   huma.Middlewares{permissions.Apply("request", "create")},
	}), func(ctx context.Context, input *dto.CreateRequestInput) (*dto.CreateRequestOutput, error) {
		request := models.NewRequest{
			Text:           input.Body.Text,
			Name:           input.Body.Name,
			Email:          input.Body.Email,
			Address:        input.Body.Address,
			Telephone:      input.Body.Telephone,
			Closed:         input.Body.Closed,
			OrganizationId: input.OrganizationId,
			CategoryId:     input.Body.CategoryId,
		}
		result, err := r.requestService.CreateRequest(ctx, request)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}
		resp := &dto.CreateRequestOutput{}
		resp.Body.Id = result
		return resp, nil
	})

	huma.Register(r.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "update-request",
		Method:        http.MethodPut,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/requests/{requestId}",
		Summary:       "Update Request",
		Tags:          []string{"Requests"},
		Middlewares:   huma.Middlewares{permissions.Apply("request", "update")},
	}), func(ctx context.Context, input *dto.UpdateRequestInput) (*struct{}, error) {

		request := models.UpdateRequest{
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
			return nil, humaUtils.NewHumaError(err)
		}

		return nil, nil
	})

	huma.Register(r.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "delete-request",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/requests/{requestId}",
		Summary:       "Delete Request",
		Tags:          []string{"Requests"},
		Middlewares:   huma.Middlewares{permissions.Apply("request", "delete")},
	}), func(ctx context.Context, input *dto.DeleteRequestInput) (*struct{}, error) {

		err := r.requestService.DeleteRequest(ctx, input.OrganizationId, input.RequestId)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		return nil, nil
	})
}
