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

type CreateKeycloakUserData struct {
	FirstName string `doc:"First name of the user to be created" example:"John"`
	LastName  string `doc:"Last name of the user to be created" example:"Doe"`
	Email     string `doc:"Email of the user to be created" example:"John.Doe@planeo.de"`
	Password  string `doc:"Initial password for the user to be set" example:"password123"`
}

func (s *OrganizationManagementService) CreateKeycloakUser(organizationId string, createUserInput CreateKeycloakUserData) error {

	createUserData := keycloak.CreateUserParams{
		FirstName: createUserInput.FirstName,
		LastName:  createUserInput.LastName,
		Email:     createUserInput.Email,
		Password:  createUserInput.Password,
	}
	err := s.KeycloakAdminClient.CreateKeycloakUser(organizationId, createUserData)

	if err != nil {
		return err
	}

	return nil
}
