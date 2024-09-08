package organization_management

import (
	"planeo/api/config"
	"planeo/api/internal/clients/keycloak"
)

type OrganizationManagementService struct {
	KeycloakAdminClient *keycloak.KeycloakAdminClient
}

func NewOrganizationManagementService() *OrganizationManagementService {
	return &OrganizationManagementService{
		KeycloakAdminClient: keycloak.NewKeycloakAdminClient(*config.Config),
	}
}

func (s *OrganizationManagementService) GetKeycloakUsers(organizationId string) ([]keycloak.KeycloakUser, error) {

	users, err := s.KeycloakAdminClient.GetKeycloakUsers(organizationId)

	if err != nil {
		return nil, err
	}

	return users, nil
}
