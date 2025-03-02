package group

import (
	"context"
	"net/http"
	"planeo/services/core/config"
	"planeo/services/core/internal/middlewares"
	"planeo/services/core/internal/setup/operations"

	"github.com/danielgtaylor/huma/v2"
)

type GroupController struct {
	api          huma.API
	groupService *GroupService
	config       *config.ApplicationConfiguration
}

func NewGroupController(api huma.API, config *config.ApplicationConfiguration) *GroupController {
	groupService := NewGroupService()
	return &GroupController{
		api:          api,
		groupService: groupService,
		config:       config,
	}
}

func (g *GroupController) InitializeRoutes() {
	huma.Register(g.api, operations.WithAuth(huma.Operation{
		OperationID: "get-group",
		Method:      http.MethodGet,
		Path:        "/{organization}/groups/{groupId}",
		Summary:     "Get Group",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(g.api, g.config, "group", "read")},
	}), func(ctx context.Context, input *GetGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result, err := g.groupService.GetGroup(input.GroupId)

		if err != nil {
			return resp, huma.Error404NotFound(err.Error())
		}
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(g.api, operations.WithAuth(huma.Operation{
		OperationID: "create-group",
		Method:      http.MethodPost,
		Path:        "/{organization}/groups",
		Summary:     "Create Group",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(g.api, g.config, "group", "create")},
	}), func(ctx context.Context, input *CreateGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := g.groupService.CreateGroup()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(g.api, operations.WithAuth(huma.Operation{
		OperationID: "update-group",
		Method:      http.MethodPut,
		Path:        "/{organization}/groups/{groupId}",
		Summary:     "Update Group",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(g.api, g.config, "group", "update")},
	}), func(ctx context.Context, input *UpdateGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := g.groupService.UpdateGroup(input.GroupId)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(g.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-group",
		Method:      http.MethodDelete,
		Path:        "/{organization}/groups/{groupId}",
		Summary:     "Delete Group",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(g.api, g.config, "group", "delete")},
	}), func(ctx context.Context, input *DeleteGroupInput) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := g.groupService.DeleteGroup(input.GroupId)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(g.api, operations.WithAuth(huma.Operation{
		OperationID: "get-groups",
		Method:      http.MethodGet,
		Path:        "/{organization}/groups",
		Summary:     "Get Groups",
		Tags:        []string{"Groups"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(g.api, g.config, "group", "read")},
	}), func(ctx context.Context, input *struct{}) (*GroupOutput, error) {
		resp := &GroupOutput{}
		result := g.groupService.GetGroups()
		resp.Body.Message = result
		return resp, nil
	})
}
