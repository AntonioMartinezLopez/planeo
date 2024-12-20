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
		Path:        "/{organization}/groups/{groupId}",
		Summary:     "Get Group",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "group", "read")},
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
		Path:        "/{organization}/groups",
		Summary:     "Create Group",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "group", "create")},
	}), func(ctx context.Context, input *CreateGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := t.groupService.CreateGroup()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "update-group",
		Method:      http.MethodPut,
		Path:        "/{organization}/groups/{groupId}",
		Summary:     "Update Group",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "group", "update")},
	}), func(ctx context.Context, input *UpdateGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := t.groupService.UpdateGroup(input.GroupId)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-group",
		Method:      http.MethodDelete,
		Path:        "/{organization}/groups/{groupId}",
		Summary:     "Delete Group",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "group", "delete")},
	}), func(ctx context.Context, input *DeleteGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := t.groupService.DeleteGroup(input.GroupId)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "get-groups",
		Method:      http.MethodGet,
		Path:        "/{organization}/groups",
		Summary:     "Get Groups",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "group", "read")},
	}), func(ctx context.Context, input *struct{}) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := t.groupService.GetGroups()
		resp.Body.Message = result
		return resp, nil
	})
}
