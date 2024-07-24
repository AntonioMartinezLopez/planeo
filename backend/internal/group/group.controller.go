package group

import (
	"context"
	"net/http"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/setup/operations"

	"github.com/danielgtaylor/huma/v2"
)

type GroupController struct {
	api          *huma.API
	groupService *GroupService
}

func NewGroupController(api *huma.API) *GroupController {
	groupService := NewGroupService()
	return &GroupController{
		api:          api,
		groupService: groupService,
	}
}

func (t *GroupController) InitializeRoutes() {
	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "get-group",
		Method:      http.MethodGet,
		Path:        "/groups/{groupId}",
		Summary:     "Get Group",
		Tags:        []string{"Group"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "read:group")},
	}), func(ctx context.Context, input *GetGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result, err := t.groupService.GetGroup(input.GroupId)

		if err != nil {
			return resp, huma.Error404NotFound(err.Error())
		}
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "create-group",
		Method:      http.MethodPost,
		Path:        "/groups",
		Summary:     "Create Group",
		Tags:        []string{"Group"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "create:group")},
	}), func(ctx context.Context, input *CreateGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := t.groupService.CreateGroup()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "update-group",
		Method:      http.MethodPut,
		Path:        "/groups/{groupId}",
		Summary:     "Update Group",
		Tags:        []string{"Group"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "update:group")},
	}), func(ctx context.Context, input *UpdateGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := t.groupService.UpdateGroup(input.GroupId)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-group",
		Method:      http.MethodDelete,
		Path:        "/groups/{groupId}",
		Summary:     "Delete Group",
		Tags:        []string{"Group"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "delete:group")},
	}), func(ctx context.Context, input *DeleteGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := t.groupService.DeleteGroup(input.GroupId)
		resp.Body.Message = result
		return resp, nil
	})

}
