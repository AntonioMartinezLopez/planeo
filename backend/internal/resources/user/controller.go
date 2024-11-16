package user

import (
	"context"
	"net/http"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/resources/user/dto"
	"planeo/api/internal/setup/operations"

	"github.com/danielgtaylor/huma/v2"
)

type UserController struct {
	api         *huma.API
	userService *UserService
}

func NewUserController(api *huma.API, userService *UserService) *UserController {
	return &UserController{
		api:         api,
		userService: userService,
	}
}

func (o *UserController) InitializeRoutes() {
	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "get-users",
		Method:      http.MethodGet,
		Path:        "/{organization}/admin/users",
		Summary:     "Get all users from organization",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "user", "read")},
	}), func(ctx context.Context, input *dto.GetUsersInput) (*dto.GetUsersOutput, error) {
		users, err := o.userService.GetUsers(input.Organization)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.GetUsersOutput{}
		response.Body.Users = users
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/{organization}/admin/users/{userId}",
		Summary:     "Get single user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "user", "read")},
	}), func(ctx context.Context, input *dto.GetUserInput) (*dto.GetUserOutput, error) {
		user, err := o.userService.GetUserById(input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.GetUserOutput{}
		response.Body.User = user
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "create-user",
		Method:      http.MethodPost,
		Path:        "/{organization}/admin/users",
		Summary:     "Create user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "user", "create")},
	}), func(ctx context.Context, input *dto.CreateUserInput) (*dto.CreateUserOutput, error) {

		err := o.userService.CreateUser(input.Organization, input.Body)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.CreateUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPut,
		Path:        "/{organization}/admin/users/{userId}",
		Summary:     "Update user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "user", "update")},
	}), func(ctx context.Context, input *dto.UpdateUserInput) (*dto.UpdateUserOutput, error) {

		err := o.userService.UpdateUser(input.UserId, input.Body)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.UpdateUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-user",
		Method:      http.MethodDelete,
		Path:        "/{organization}/admin/users/{userId}",
		Summary:     "Delete user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "user", "delete")},
	}), func(ctx context.Context, input *dto.DeleteUserInput) (*dto.DeleteUserOutput, error) {

		err := o.userService.DeleteUser(input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.DeleteUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "Assign-user-roles",
		Method:      http.MethodPut,
		Path:        "/{organization}/admin/users/{userId}/roles",
		Summary:     "Assign roles to a user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "user", "update")},
	}), func(ctx context.Context, input *dto.PutUserRolesInput) (*dto.PutUserRoleOutput, error) {

		err := o.userService.AssignRoles(input.Body.Roles, input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.PutUserRoleOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "get-roles",
		Method:      http.MethodGet,
		Path:        "/{organization}/admin/roles",
		Summary:     "Get roles",
		Tags:        []string{"Roles"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "role", "read")},
	}), func(ctx context.Context, input *dto.GetRolesInput) (*dto.GetRolesOutput, error) {

		roles, err := o.userService.GetAvailableRoles()

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.GetRolesOutput{}
		response.Body.Roles = roles
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "get-basic-user-information",
		Method:      http.MethodGet,
		Path:        "/{organization}/users",
		Summary:     "Get users",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "userinfo", "read")},
	}), func(ctx context.Context, input *dto.GetUsersInput) (*dto.GetUserInfoOutput, error) {

		users, err := o.userService.GetUsersInformation(input.Organization)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.GetUserInfoOutput{}
		response.Body.Users = users
		return response, nil
	})
}
