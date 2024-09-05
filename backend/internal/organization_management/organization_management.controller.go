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
		OperationID: "get-organization-keycloak-users",
		Method:      http.MethodGet,
		Path:        "/{organization}/management/keycloak/users",
		Summary:     "Get all users from keycloak",
		Tags:        []string{"Management"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(*o.api, "organization", "manage")},
	}), func(ctx context.Context, input *GetKeycloakUserInput) (*KeycloakUserOutput, error) {
		users, err := o.organizationManagementService.GetKeycloakUsers(input.Organization)

		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		response := &KeycloakUserOutput{}
		response.Body.Users = users
		return response, nil
	})
}
