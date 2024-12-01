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

func (controller *UserController) InitializeRoutes() {
	huma.Register(*controller.api, operations.WithAuth(huma.Operation{
		OperationID: "get-users",
		Method:      http.MethodGet,
		Path:        "/{organization}/admin/users",
		Summary:     "[Admin] Get all users from organization",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*controller.api, "user", "read")},
	}), func(ctx context.Context, input *dto.GetUsersInput) (*dto.GetUsersOutput, error) {
		users, err := controller.userService.GetUsers(input.Organization, input.Sync)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.GetUsersOutput{}
		response.Body.Users = users
		return response, nil
	})

	huma.Register(*controller.api, operations.WithAuth(huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/{organization}/admin/users/{userId}",
		Summary:     "[Admin] Get single user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*controller.api, "user", "read")},
	}), func(ctx context.Context, input *dto.GetUserInput) (*dto.GetUserOutput, error) {
		user, err := controller.userService.GetUserById(input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.GetUserOutput{}
		response.Body.User = user
		return response, nil
	})

	huma.Register(*controller.api, operations.WithAuth(huma.Operation{
		OperationID: "create-user",
		Method:      http.MethodPost,
		Path:        "/{organization}/admin/users",
		Summary:     "[Admin] Create user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*controller.api, "user", "create")},
	}), func(ctx context.Context, input *dto.CreateUserInput) (*dto.CreateUserOutput, error) {

		err := controller.userService.CreateUser(input.Organization, input.Body)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.CreateUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*controller.api, operations.WithAuth(huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPut,
		Path:        "/{organization}/admin/users/{userId}",
		Summary:     "[Admin] Update user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*controller.api, "user", "update")},
	}), func(ctx context.Context, input *dto.UpdateUserInput) (*dto.UpdateUserOutput, error) {

		err := controller.userService.UpdateUser(input.UserId, input.Body)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.UpdateUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*controller.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-user",
		Method:      http.MethodDelete,
		Path:        "/{organization}/admin/users/{userId}",
		Summary:     "[Admin] Delete user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*controller.api, "user", "delete")},
	}), func(ctx context.Context, input *dto.DeleteUserInput) (*dto.DeleteUserOutput, error) {

		err := controller.userService.DeleteUser(input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.DeleteUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*controller.api, operations.WithAuth(huma.Operation{
		OperationID: "Assign-user-roles",
		Method:      http.MethodPut,
		Path:        "/{organization}/admin/users/{userId}/roles",
		Summary:     "[Admin] Assign roles to a user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*controller.api, "user", "update")},
	}), func(ctx context.Context, input *dto.PutUserRolesInput) (*dto.PutUserRoleOutput, error) {

		err := controller.userService.AssignRoles(input.Body.Roles, input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.PutUserRoleOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*controller.api, operations.WithAuth(huma.Operation{
		OperationID: "get-roles",
		Method:      http.MethodGet,
		Path:        "/{organization}/admin/roles",
		Summary:     "[Admin] Get roles",
		Tags:        []string{"Roles"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*controller.api, "role", "read")},
	}), func(ctx context.Context, input *dto.GetRolesInput) (*dto.GetRolesOutput, error) {

		roles, err := controller.userService.GetAvailableRoles()

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.GetRolesOutput{}
		response.Body.Roles = roles
		return response, nil
	})

	huma.Register(*controller.api, operations.WithAuth(huma.Operation{
		OperationID: "get-basic-user-information",
		Method:      http.MethodGet,
		Path:        "/{organization}/users",
		Summary:     "Get basic information of users",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*controller.api, "userinfo", "read")},
	}), func(ctx context.Context, input *dto.GetUserInfoInput) (*dto.GetUserInfoOutput, error) {

		users, err := controller.userService.GetUsersInformation(input.Organization)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &dto.GetUserInfoOutput{}
		response.Body.Users = users
		return response, nil
	})
}
