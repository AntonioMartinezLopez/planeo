package user

import (
	"context"
	"net/http"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/setup/operations"

	"github.com/danielgtaylor/huma/v2"
)

type UserController struct {
	api         *huma.API
	userService *UserService
}

func NewUserController(api *huma.API) *UserController {
	userService := NewUserService()
	return &UserController{
		api:         api,
		userService: userService,
	}
}

func (t *UserController) InitializeRoutes() {
	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/{organization}/users/{userId}",
		Summary:     "Get User",
		Tags:        []string{"Users"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "user", "read")},
	}), func(ctx context.Context, input *GetUserInput) (*UserOutput, error) {
		resp := &UserOutput{}
		result, err := t.userService.GetUser(input.UserId)

		if err != nil {
			return resp, huma.Error404NotFound(err.Error())
		}
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "create-user",
		Method:      http.MethodPost,
		Path:        "/{organization}/users",
		Summary:     "Create User",
		Tags:        []string{"Users"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "user", "create")},
	}), func(ctx context.Context, input *CreateUserInput) (*UserOutput, error) {
		resp := &UserOutput{}
		result := t.userService.CreateUser()
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPut,
		Path:        "/{organization}/users/{userId}",
		Summary:     "Update User",
		Tags:        []string{"Users"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "user", "update")},
	}), func(ctx context.Context, input *UpdateUserInput) (*UserOutput, error) {
		resp := &UserOutput{}
		result := t.userService.UpdateUser(input.UserId)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-user",
		Method:      http.MethodDelete,
		Path:        "/{organization}/users/{userId}",
		Summary:     "Delete User",
		Tags:        []string{"Users"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "user", "delete")},
	}), func(ctx context.Context, input *DeleteUserInput) (*UserOutput, error) {
		resp := &UserOutput{}
		result := t.userService.DeleteUser(input.UserId)
		resp.Body.Message = result
		return resp, nil
	})

	huma.Register(*t.api, operations.WithAuth(huma.Operation{
		OperationID: "get-users",
		Method:      http.MethodGet,
		Path:        "/{organization}/users",
		Summary:     "Get Users",
		Tags:        []string{"Users"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*t.api, "user", "read")},
	}), func(ctx context.Context, input *struct{}) (*UserOutput, error) {
		resp := &UserOutput{}
		result := t.userService.GetUsers()
		resp.Body.Message = result
		return resp, nil
	})
}
