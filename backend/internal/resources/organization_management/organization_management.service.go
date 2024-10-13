package organization_management

import (
	"errors"
	"planeo/api/config"
	"planeo/api/internal/clients/keycloak"
	"slices"
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

	// assign default role
	client, err := s.KeycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return err
	}

	clientRoles, err := s.KeycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		return err
	}

	index := slices.IndexFunc(clientRoles, func(role keycloak.KeycloakClientRole) bool {
		return role.Name == keycloak.User.String()
	})

	if index == -1 {
		return errors.New("client role not found")
	}

	role := clientRoles[index]

	user, err := s.KeycloakAdminClient.GetKeycloakUser(createUserData.Email)

	if err != nil {
		return err
	}

	roleAssignError := s.KeycloakAdminClient.AssignKeycloakClientRoleToUser(client.Uuid, role, user.Id)

	if roleAssignError != nil {
		return roleAssignError
	}

	return nil
}

func (s *OrganizationManagementService) GetAvailableRoles() ([]keycloak.KeycloakClientRole, error) {
	clientRoles, err := s.KeycloakAdminClient.GetKeycloakClientRoles(config.Config.KcOauthClientID)

	if err != nil {
		return nil, err
	}

	return clientRoles, nil
}

func (s *OrganizationManagementService) DeleteKeycloakUser(organizationId string)  {}
func (s *OrganizationManagementService) AssignRole(roleName string, userId string) {}
func (s *OrganizationManagementService) GetUser(email string)                      {}
