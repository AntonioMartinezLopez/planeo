package organization_management

import (
	keycloak "planeo/api/internal/resources/organization_management/keycloak_queries"
)

type OrganizationManagementService struct {
}

func NewOrganizationManagementService() *OrganizationManagementService {
	return &OrganizationManagementService{}
}

func (s *OrganizationManagementService) GetKeycloakUsers(organizationId string) ([]keycloak.KeycloakUser, error) {

	// TODO: the token needs to be cached to reduce requests
	token, err := keycloak.AuthenticateAdmin()

	if err != nil {
		return nil, err
	}

	// TODO: group information should be cached
	users, err := keycloak.GetKeycloakUsers(token, organizationId)

	if err != nil {
		return nil, err
	}

	return users, nil
}
