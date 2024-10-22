package organization_management

import (
	"context"
	"net/http"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/setup/operations"

	"github.com/danielgtaylor/huma/v2"
)

type OrganisationManagementController struct {
	api                           *huma.API
	organizationManagementService *OrganizationManagementService
}

func NewOrganisationManagementController(api *huma.API) *OrganisationManagementController {
	organizationManagementService := NewOrganizationManagementService()
	return &OrganisationManagementController{
		api:                           api,
		organizationManagementService: organizationManagementService,
	}
}

func (o *OrganisationManagementController) InitializeRoutes() {
	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "get-users",
		Method:      http.MethodGet,
		Path:        "/{organization}/management/users",
		Summary:     "Get all users from organization",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *GetUsersInput) (*GetUsersOutput, error) {
		users, err := o.organizationManagementService.GetUsers(input.Organization)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &GetUsersOutput{}
		response.Body.Users = users
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/{organization}/management/users/{userId}",
		Summary:     "Get single user",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *GetUserInput) (*GetUserOutput, error) {
		user, err := o.organizationManagementService.GetUserById(input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &GetUserOutput{}
		response.Body.User = &UserWithRoles{User: *user, Roles: []Role{}}
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "create-user",
		Method:      http.MethodPost,
		Path:        "/{organization}/management/users",
		Summary:     "Create user",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *CreateUserInput) (*CreateUserOutput, error) {

		err := o.organizationManagementService.CreateUser(input.Organization, input.Body)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &CreateUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPut,
		Path:        "/{organization}/management/users/{userId}",
		Summary:     "Update user",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *UpdateUserInput) (*UpdateUserOutput, error) {

		err := o.organizationManagementService.UpdateUser(input.UserId, input.Body)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &UpdateUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-user",
		Method:      http.MethodDelete,
		Path:        "/{organization}/management/users/{userId}",
		Summary:     "Delete user",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *DeleteUserInput) (*DeleteUserOutput, error) {

		err := o.organizationManagementService.DeleteUser(input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &DeleteUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "Assign-user-roles",
		Method:      http.MethodPut,
		Path:        "/{organization}/management/users/{userId}/roles",
		Summary:     "Assign roles to a user",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *PutUserRolesInput) (*PutUserRoleOutput, error) {

		err := o.organizationManagementService.AssignRoles(input.Body.Roles, input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &PutUserRoleOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "get-roles",
		Method:      http.MethodGet,
		Path:        "/{organization}/management/roles",
		Summary:     "Get roles",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *GetRolesInput) (*GetRolesOutput, error) {

		roles, err := o.organizationManagementService.GetAvailableRoles()

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &GetRolesOutput{}
		response.Body.Roles = roles
		return response, nil
	})
}
