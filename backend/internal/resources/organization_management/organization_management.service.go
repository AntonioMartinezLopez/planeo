package organization_management

import (
	"errors"
	"fmt"
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

func (s *OrganizationManagementService) GetUsers(organizationId string) ([]keycloak.KeycloakUser, error) {

	users, err := s.KeycloakAdminClient.GetKeycloakUsers(organizationId)

	if err != nil {
		return nil, err
	}

	return users, nil
}

type CreateUserData struct {
	FirstName string `json:"firstName" doc:"First name of the user to be created" example:"John"`
	LastName  string `json:"lastName" doc:"Last name of the user to be created" example:"Doe"`
	Email     string `json:"email" doc:"Email of the user to be created" example:"John.Doe@planeo.de"`
	Password  string `json:"password" doc:"Initial password for the user to be set" example:"password123"`
}

func (s *OrganizationManagementService) CreateUser(organizationId string, createUserInput CreateUserData) error {

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

	user, err := s.KeycloakAdminClient.GetKeycloakUserByEmail(createUserData.Email)

	if err != nil {
		return err
	}

	roleAssignError := s.KeycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, []keycloak.KeycloakClientRole{role}, user.Id)

	if roleAssignError != nil {
		return roleAssignError
	}

	return nil
}

func (s *OrganizationManagementService) GetAvailableRoles() ([]keycloak.KeycloakClientRole, error) {
	client, err := s.KeycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return nil, err
	}
	clientRoles, err := s.KeycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		return nil, err
	}

	return clientRoles, nil
}

func (s *OrganizationManagementService) DeleteUser(userId string) error {
	err := s.KeycloakAdminClient.DeleteKeycloakUser(userId)

	if err != nil {
		return err
	}

	return nil
}

func (s *OrganizationManagementService) AssignRoles(roles []PutUserRoleInputBody, userId string) error {

	client, err := s.KeycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return err
	}

	userRoles, erro := s.KeycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if erro != nil {
		return erro
	}

	rolesToDelete := slices.DeleteFunc(userRoles, func(userRole keycloak.KeycloakClientRole) bool {
		i := slices.IndexFunc(roles, func(role PutUserRoleInputBody) bool {
			return role.Id == userRole.Id
		})

		return i != -1
	})

	fmt.Println(rolesToDelete)
	deleteError := s.KeycloakAdminClient.DeleteKeycloakUserClientRoles(client.Uuid, rolesToDelete, userId)

	if deleteError != nil {
		return deleteError
	}

	var keycloakRoles = []keycloak.KeycloakClientRole{}
	for _, role := range roles {
		keycloakRoles = append(keycloakRoles, keycloak.KeycloakClientRole{Id: role.Id, Name: role.Name})
	}

	s.KeycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, keycloakRoles, userId)

	return nil
}

func (s *OrganizationManagementService) GetUser(email string) (*keycloak.KeycloakUser, error) {
	user, err := s.KeycloakAdminClient.GetKeycloakUserByEmail(email)

	if err != nil {

		return nil, err
	}

	return user, nil
}

func (s *OrganizationManagementService) GetUserById(userId string) (*keycloak.KeycloakUser, error) {
	user, err := s.KeycloakAdminClient.GetKeycloakUserById(userId)

	if err != nil {

		return nil, err
	}

	return user, nil
}
