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
		OperationID: "get-keycloak-users",
		Method:      http.MethodGet,
		Path:        "/{organization}/management/keycloak/users",
		Summary:     "Get all users from keycloak",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *GetKeycloakUsersInput) (*KeycloakUsersOutput, error) {
		users, err := o.organizationManagementService.GetKeycloakUsers(input.Organization)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &KeycloakUsersOutput{}
		response.Body.Users = users
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "get-keycloak-user",
		Method:      http.MethodGet,
		Path:        "/{organization}/management/keycloak/user/{userId}",
		Summary:     "Get user from keycloak",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *GetKeycloakUserInput) (*KeycloakUserOutput, error) {
		user, err := o.organizationManagementService.KeycloakAdminClient.GetKeycloakUserById(input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &KeycloakUserOutput{}
		response.Body.User = user
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "create-keycloak-user",
		Method:      http.MethodPost,
		Path:        "/{organization}/management/keycloak/user",
		Summary:     "Create Keycloak user",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *CreateKeycloakUserInput) (*CreateKeycloakUserOutput, error) {

		err := o.organizationManagementService.CreateKeycloakUser(input.Organization, input.Body)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &CreateKeycloakUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "delete-keycloak-user",
		Method:      http.MethodDelete,
		Path:        "/{organization}/management/keycloak/user/{userId}",
		Summary:     "Delete Keycloak user",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *DeleteKeycloakUserInput) (*DeleteKeycloakUserOutput, error) {

		err := o.organizationManagementService.DeleteKeycloakUser(input.UserId)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &DeleteKeycloakUserOutput{}
		response.Body.Success = true
		return response, nil
	})

	huma.Register(*o.api, operations.WithAuth(huma.Operation{
		OperationID: "get-keycloak-roles",
		Method:      http.MethodGet,
		Path:        "/{organization}/management/keycloak/roles",
		Summary:     "Get Keycloak roles",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *GetKeycloakRolesInput) (*GetKeycloakRolesOutput, error) {

		roles, err := o.organizationManagementService.GetAvailableRoles()

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &GetKeycloakRolesOutput{}
		response.Body.Roles = roles
		return response, nil
	})
}
