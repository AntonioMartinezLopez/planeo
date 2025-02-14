package request

import (
	"context"
	"net/http"
	"planeo/api/config"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/resources/request/dto"
	"planeo/api/internal/setup/operations"
	"planeo/api/internal/utils/huma_utils"

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

func (t *RequestController) InitializeRoutes() {
	huma.Register(t.api, operations.WithAuth(huma.Operation{
		OperationID: "get-requests",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/requests",
		Summary:     "Get Requests",
		Tags:        []string{"Requests"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(t.api, t.config, "task", "read")},
	}), func(ctx context.Context, input *dto.GetRequestsInput) (*dto.GetRequestsOutput, error) {

		result, err := t.requestService.GetRequests(ctx, input.OrganizationId, input.Cursor, input.PageSize, input.GetClosed)

		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}

		resp := &dto.GetRequestsOutput{}
		resp.Body.Requests = result
		resp.Body.Limit = input.PageSize

		if len(result) > 0 {
			resp.Body.NextCursor = result[len(result)-1].Id
		}

		return resp, nil
	})

	huma.Register(t.api, operations.WithAuth(huma.Operation{
		OperationID: "create-request",
		Method:      http.MethodPost,
		Path:        "/organizations/{organizationId}/requests",
		Summary:     "Create Request",
		Tags:        []string{"Requests"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(t.api, t.config, "task", "create")},
	}), func(ctx context.Context, input *dto.CreateRequestInput) (*dto.CreateRequestOutput, error) {
		resp := &dto.CreateRequestOutput{}
		result := t.requestService.CreateRequest(ctx, input.OrganizationId, input.Body)

		if result != nil {
			return nil, huma_utils.NewHumaError(result)
		}

		resp.Body.Success = true
		return resp, nil
	})

	huma.Register(t.api, operations.WithAuth(huma.Operation{
		OperationID: "update-request",
		Method:      http.MethodPut,
		Path:        "/organizations/{organizationId}/requests/{requestId}",
		Summary:     "Update Request",
		Tags:        []string{"Requests"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(t.api, t.config, "task", "update")},
	}), func(ctx context.Context, input *dto.UpdateRequestInput) (*dto.UpdateRequestOutput, error) {

		err := t.requestService.UpdateRequest(ctx, input.OrganizationId, input.RequestId, input.Body)

		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}

		resp := &dto.UpdateRequestOutput{}
		resp.Body.Success = true
		return resp, nil
	})

	huma.Register(t.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-request",
		Method:      http.MethodDelete,
		Path:        "/organizations/{organizationId}/requests/{requestId}",
		Summary:     "Delete Request",
		Tags:        []string{"Requests"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(t.api, t.config, "task", "delete")},
	}), func(ctx context.Context, input *dto.DeleteRequestInput) (*dto.DeleteRequestOutput, error) {
		resp := &dto.DeleteRequestOutput{}
		// result := t.requestService.DeleteTask(input.TaskId)
		resp.Body.Success = true
		return resp, nil
	})
}
