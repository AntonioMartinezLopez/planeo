package user

import (
	"context"
	"net/http"
	"planeo/libs/middlewares"
	"planeo/services/core/config"
	"planeo/services/core/internal/resources/user/dto"
	"planeo/services/core/internal/setup/operations"
	humaUtils "planeo/services/core/internal/utils/huma_utils"

	"github.com/danielgtaylor/huma/v2"
)

type UserController struct {
	api         huma.API
	userService *UserService
	config      *config.ApplicationConfiguration
}

func NewUserController(api huma.API, config *config.ApplicationConfiguration, userService *UserService) *UserController {
	return &UserController{
		api:         api,
		userService: userService,
		config:      config,
	}
}

func (u *UserController) InitializeRoutes() {
	permissions := middlewares.NewPermissionMiddlewareConfig(u.api, u.config.OauthIssuerUrl(), u.config.KcOauthClientID)
	huma.Register(u.api, operations.WithAuth(huma.Operation{
		OperationID: "get-users",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/iam/users",
		Summary:     "[Admin] Get all users from organization",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{permissions.Apply("user", "read")},
	}), func(ctx context.Context, input *dto.GetUsersInput) (*dto.GetUsersOutput, error) {
		users, err := u.userService.GetUsers(ctx, input.OrganizationId, input.Sync)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		response := &dto.GetUsersOutput{}
		response.Body.Users = users
		return response, nil
	})

	huma.Register(u.api, operations.WithAuth(huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/iam/users/{iamUserId}",
		Summary:     "[Admin] Get single user",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{permissions.Apply("user", "read")},
	}), func(ctx context.Context, input *dto.GetUserInput) (*dto.GetUserOutput, error) {
		user, err := u.userService.GetUserById(ctx, input.OrganizationId, input.IamUserId)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		response := &dto.GetUserOutput{}
		response.Body.User = user
		return response, nil
	})

	huma.Register(u.api, operations.WithAuth(huma.Operation{
		OperationID:   "create-user",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/organizations/{organizationId}/iam/users",
		Summary:       "[Admin] Create user",
		Tags:          []string{"User"},
		Middlewares:   huma.Middlewares{permissions.Apply("user", "create")},
	}), func(ctx context.Context, input *dto.CreateUserInput) (*struct{}, error) {

		err := u.userService.CreateUser(ctx, input.OrganizationId, input.Body)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		return nil, nil
	})

	huma.Register(u.api, operations.WithAuth(huma.Operation{
		OperationID:   "update-user",
		Method:        http.MethodPut,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/iam/users/{iamUserId}",
		Summary:       "[Admin] Update user",
		Tags:          []string{"User"},
		Middlewares:   huma.Middlewares{permissions.Apply("user", "update")},
	}), func(ctx context.Context, input *dto.UpdateUserInput) (*struct{}, error) {

		err := u.userService.UpdateUser(ctx, input.OrganizationId, input.IamUserId, input.Body)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		return nil, nil
	})

	huma.Register(u.api, operations.WithAuth(huma.Operation{
		OperationID:   "delete-user",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/iam/users/{iamUserId}",
		Summary:       "[Admin] Delete user",
		Tags:          []string{"User"},
		Middlewares:   huma.Middlewares{permissions.Apply("user", "delete")},
	}), func(ctx context.Context, input *dto.DeleteUserInput) (*struct{}, error) {

		err := u.userService.DeleteUser(ctx, input.OrganizationId, input.IamUserId)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		return nil, nil
	})

	huma.Register(u.api, operations.WithAuth(huma.Operation{
		OperationID:   "Assign-user-roles",
		Method:        http.MethodPut,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/iam/users/{iamUserId}/roles",
		Summary:       "[Admin] Assign roles to a user",
		Tags:          []string{"User"},
		Middlewares:   huma.Middlewares{permissions.Apply("user", "update")},
	}), func(ctx context.Context, input *dto.PutUserRolesInput) (*struct{}, error) {

		err := u.userService.AssignRoles(ctx, input.OrganizationId, input.IamUserId, input.Body)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		return nil, nil
	})

	huma.Register(u.api, operations.WithAuth(huma.Operation{
		OperationID: "get-roles",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/iam/roles",
		Summary:     "[Admin] Get roles",
		Tags:        []string{"Roles"},
		Middlewares: huma.Middlewares{permissions.Apply("role", "read")},
	}), func(ctx context.Context, input *dto.GetRolesInput) (*dto.GetRolesOutput, error) {

		roles, err := u.userService.GetAvailableRoles(ctx)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		response := &dto.GetRolesOutput{}
		response.Body.Roles = roles
		return response, nil
	})

	huma.Register(u.api, operations.WithAuth(huma.Operation{
		OperationID: "get-basic-user-information",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/users",
		Summary:     "Get basic information of users",
		Tags:        []string{"User"},
		Middlewares: huma.Middlewares{permissions.Apply("userinfo", "read")},
	}), func(ctx context.Context, input *dto.GetUserInfoInput) (*dto.GetUserInfoOutput, error) {

		users, err := u.userService.GetUsersInformation(ctx, input.OrganizationId)

		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		response := &dto.GetUserInfoOutput{}
		response.Body.Users = users
		return response, nil
	})
}
