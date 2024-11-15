package user

import (
	"context"
	"errors"
	"planeo/api/config"
	"planeo/api/internal/clients/keycloak"
	"slices"
)

type UserService struct {
	keycloakAdminClient *keycloak.KeycloakAdminClient
	userRepositry       *UserRepository
}

func NewUserService(userRepository *UserRepository) *UserService {
	return &UserService{
		keycloakAdminClient: keycloak.NewKeycloakAdminClient(*config.Config),
		userRepositry:       userRepository,
	}
}

func (s *UserService) GetUsers(organizationId string) ([]User, error) {

	keycloakUsers, err := s.keycloakAdminClient.GetKeycloakUsers(organizationId)

	if err != nil {
		return nil, err
	}

	users := []User{}
	for _, keycloakUser := range keycloakUsers {
		users = append(users, fromKeycloakUser(&keycloakUser))
	}

	return users, nil
}

type CreateUserData struct {
	FirstName string `json:"firstName" doc:"First name of the user to be created" example:"John"`
	LastName  string `json:"lastName" doc:"Last name of the user to be created" example:"Doe"`
	Email     string `json:"email" doc:"Email of the user to be created" example:"John.Doe@planeo.de"`
	Password  string `json:"password" doc:"Initial password for the user to be set" example:"password123"`
}

func (s *UserService) CreateUser(organizationId string, createUserInput CreateUserData) error {

	createUserData := keycloak.CreateUserParams{
		FirstName: createUserInput.FirstName,
		LastName:  createUserInput.LastName,
		Email:     createUserInput.Email,
		Password:  createUserInput.Password,
	}
	err := s.keycloakAdminClient.CreateKeycloakUser(organizationId, createUserData)

	if err != nil {
		return err
	}

	// assign default role
	client, err := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return err
	}

	clientRoles, err := s.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

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

	user, err := s.keycloakAdminClient.GetKeycloakUserByEmail(createUserData.Email)

	if err != nil {
		return err
	}

	roleAssignError := s.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, []keycloak.KeycloakClientRole{role}, user.Id)

	if roleAssignError != nil {
		return roleAssignError
	}

	return nil
}

func (s *UserService) DeleteUser(userId string) error {
	err := s.keycloakAdminClient.DeleteKeycloakUser(userId)

	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) UpdateUser(userId string, user User) error {

	updateKeycloakUserParams := keycloak.UpdateUserParams{
		Id:              userId,
		Userame:         user.Username,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		Email:           user.Email,
		Enabled:         user.Enabled,
		EmailVerified:   user.EmailVerified,
		Totp:            user.Totp,
		RequiredActions: mapToKeycloakActions(user.RequiredActions),
	}

	err := s.keycloakAdminClient.UpdateKeycloakUser(userId, updateKeycloakUserParams)

	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) GetAvailableRoles() ([]Role, error) {
	client, err := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return nil, err
	}
	keycloakClientRoles, err := s.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		return nil, err
	}

	roles := []Role{}
	for _, keycloakClientRole := range keycloakClientRoles {
		roles = append(roles, fromKeycloakClientRole(&keycloakClientRole))
	}

	return roles, nil
}

func (s *UserService) AssignRoles(roles []PutUserRoleInputBody, userId string) error {

	client, err := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return err
	}

	userRoles, erro := s.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if erro != nil {
		return erro
	}

	rolesToDelete := slices.DeleteFunc(userRoles, func(userRole keycloak.KeycloakClientRole) bool {
		i := slices.IndexFunc(roles, func(role PutUserRoleInputBody) bool {
			return role.Id == userRole.Id
		})

		return i != -1
	})

	deleteError := s.keycloakAdminClient.DeleteKeycloakUserClientRoles(client.Uuid, rolesToDelete, userId)

	if deleteError != nil {
		return deleteError
	}

	var keycloakRoles = []keycloak.KeycloakClientRole{}
	for _, role := range roles {
		keycloakRoles = append(keycloakRoles, keycloak.KeycloakClientRole{Id: role.Id, Name: role.Name})
	}

	s.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, keycloakRoles, userId)

	return nil
}

func (s *UserService) GetUserById(userId string) (*UserWithRoles, error) {
	keycloakUser, err := s.keycloakAdminClient.GetKeycloakUserById(userId)

	if err != nil {
		return nil, err
	}

	user := fromKeycloakUser(keycloakUser)

	client, err := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return nil, err
	}

	userRoles, err := s.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		return nil, err
	}

	roles := []Role{}
	for _, keycloakClientRole := range userRoles {
		roles = append(roles, fromKeycloakClientRole(&keycloakClientRole))
	}

	userWithRoles := UserWithRoles{User: user, Roles: roles}

	return &userWithRoles, nil
}

func (s *UserService) GetUsersInformation(ctx context.Context, organizationId string) ([]BasicUserInformation, error) {
	return s.userRepositry.GetUsersInformation(ctx, organizationId)
}
