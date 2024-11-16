package user

import (
	"errors"
	"planeo/api/config"
	"planeo/api/internal/clients/keycloak"
	"planeo/api/internal/resources/user/acl"
	"planeo/api/internal/resources/user/dto"
	"planeo/api/internal/resources/user/models"
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

func (s *UserService) GetUsers(organizationId string) ([]models.User, error) {

	keycloakUsers, err := s.keycloakAdminClient.GetKeycloakUsers(organizationId)

	if err != nil {
		return nil, err
	}

	users := []models.User{}
	for _, keycloakUser := range keycloakUsers {
		users = append(users, acl.FromKeycloakUser(&keycloakUser))
	}

	return users, nil
}

func (s *UserService) CreateUser(organizationId string, createUserInput dto.CreateUserInputBody) error {

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

func (s *UserService) UpdateUser(userId string, user models.User) error {

	updateKeycloakUserParams := keycloak.UpdateUserParams{
		Id:              userId,
		Userame:         user.Username,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		Email:           user.Email,
		Enabled:         user.Enabled,
		EmailVerified:   user.EmailVerified,
		Totp:            user.Totp,
		RequiredActions: acl.MapToKeycloakActions(user.RequiredActions),
	}

	err := s.keycloakAdminClient.UpdateKeycloakUser(userId, updateKeycloakUserParams)

	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) GetAvailableRoles() ([]models.Role, error) {
	client, err := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return nil, err
	}
	keycloakClientRoles, err := s.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		return nil, err
	}

	roles := []models.Role{}
	for _, keycloakClientRole := range keycloakClientRoles {
		roles = append(roles, acl.FromKeycloakClientRole(&keycloakClientRole))
	}

	return roles, nil
}

func (s *UserService) AssignRoles(roles []dto.PutUserRoleInputBody, userId string) error {

	client, err := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return err
	}

	userRoles, erro := s.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if erro != nil {
		return erro
	}

	rolesToDelete := slices.DeleteFunc(userRoles, func(userRole keycloak.KeycloakClientRole) bool {
		i := slices.IndexFunc(roles, func(role dto.PutUserRoleInputBody) bool {
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

func (s *UserService) GetUserById(userId string) (*models.UserWithRoles, error) {
	keycloakUser, err := s.keycloakAdminClient.GetKeycloakUserById(userId)

	if err != nil {
		return nil, err
	}

	user := acl.FromKeycloakUser(keycloakUser)

	client, err := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return nil, err
	}

	userRoles, err := s.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		return nil, err
	}

	roles := []models.Role{}
	for _, keycloakClientRole := range userRoles {
		roles = append(roles, acl.FromKeycloakClientRole(&keycloakClientRole))
	}

	userWithRoles := models.UserWithRoles{User: user, Roles: roles}

	return &userWithRoles, nil
}

func (s *UserService) GetUsersInformation(organizationId string) ([]models.BasicUserInformation, error) {
	return s.userRepositry.GetUsersInformation(organizationId)
}
