package user

import (
	"planeo/api/config"
	"planeo/api/internal/clients/keycloak"
	appError "planeo/api/internal/errors"
	"planeo/api/internal/resources/user/acl"
	"planeo/api/internal/resources/user/dto"
	"planeo/api/internal/resources/user/models"
	"planeo/api/internal/resources/user/policies"
	"slices"
)

type KeycloakAdminClientInterface interface {
	GetKeycloakUsers(groupId string) ([]keycloak.KeycloakUser, error)
	CreateKeycloakUser(groupId string, params keycloak.CreateUserParams) error
	GetKeycloakClient(clientID string) (*keycloak.KeycloakClient, error)
	GetKeycloakClientRoles(clientUuid string) ([]keycloak.KeycloakClientRole, error)
	GetKeycloakUserByEmail(email string) (*keycloak.KeycloakUser, error)
	AddKeycloakClientRoleToUser(clientUuid string, roles []keycloak.KeycloakClientRole, userId string) error
	DeleteKeycloakUser(userId string) error
	UpdateKeycloakUser(userId string, params keycloak.UpdateUserParams) error
	GetKeycloakUserClientRoles(clientUuid, userId string) ([]keycloak.KeycloakClientRole, error)
	DeleteKeycloakUserClientRoles(clientUuid string, roles []keycloak.KeycloakClientRole, userId string) error
	GetKeycloakUserById(userId string) (*keycloak.KeycloakUser, error)
	GetKeycloakUserGroups(userId string) ([]keycloak.KeycloakGroup, error)
}

type UserRepositoryInterface interface {
	GetUsersInformation(organizationId string) ([]models.BasicUserInformation, error)
	SyncUsers(organizationId string, users []models.User) error
}

type UserService struct {
	keycloakAdminClient KeycloakAdminClientInterface
	userRepository      UserRepositoryInterface
}

func NewUserService(userRepository UserRepositoryInterface, keycloakAdminClient KeycloakAdminClientInterface) *UserService {
	return &UserService{
		keycloakAdminClient: keycloakAdminClient,
		userRepository:      userRepository,
	}
}

func (s *UserService) GetUsers(organizationId string, sync bool) ([]models.User, error) {

	keycloakUsers, err := s.keycloakAdminClient.GetKeycloakUsers(organizationId)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	users := []models.User{}
	for _, keycloakUser := range keycloakUsers {
		users = append(users, acl.FromKeycloakUser(&keycloakUser))
	}

	if sync {
		err := s.userRepository.SyncUsers(organizationId, users)

		if err != nil {
			return nil, appError.New(appError.InternalError, "Something went wrong", err)
		}
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
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	// assign default role
	client, err := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	clientRoles, err := s.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	index := slices.IndexFunc(clientRoles, func(role keycloak.KeycloakClientRole) bool {
		return role.Name == keycloak.User.String()
	})

	if index == -1 {
		return appError.New(appError.EntityNotFound, "client role not found", err)
	}

	role := clientRoles[index]

	user, err := s.keycloakAdminClient.GetKeycloakUserByEmail(createUserData.Email)

	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	roleAssignError := s.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, []keycloak.KeycloakClientRole{role}, user.Id)

	if roleAssignError != nil {
		return roleAssignError
	}

	return nil
}

func (s *UserService) DeleteUser(organizationId string, userId string) error {

	result := policies.UserInOrganisation(s.keycloakAdminClient, organizationId, userId)

	if !result {
		return appError.New(appError.EntityNotFound, "User not found in organization")
	}

	keycloakErr := s.keycloakAdminClient.DeleteKeycloakUser(userId)

	if keycloakErr != nil {
		return appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}

	return nil
}

func (s *UserService) UpdateUser(organizationId string, userId string, user dto.UpdateUserInputBody) error {

	result := policies.UserInOrganisation(s.keycloakAdminClient, organizationId, userId)

	if !result {
		return appError.New(appError.EntityNotFound, "User not found in organization")
	}

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

	keycloakErr := s.keycloakAdminClient.UpdateKeycloakUser(userId, updateKeycloakUserParams)

	if keycloakErr != nil {
		return appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}

	return nil
}

func (s *UserService) GetAvailableRoles() ([]models.Role, error) {
	client, keycloakErr := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if keycloakErr != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}
	keycloakClientRoles, keycloakErr := s.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if keycloakErr != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}

	roles := []models.Role{}
	for _, keycloakClientRole := range keycloakClientRoles {
		roles = append(roles, acl.FromKeycloakClientRole(&keycloakClientRole))
	}

	return roles, nil
}

func (s *UserService) AssignRoles(organizationId string, userId string, roles []dto.PutUserRoleInputBody) error {

	result := policies.UserInOrganisation(s.keycloakAdminClient, organizationId, userId)

	if !result {
		return appError.New(appError.EntityNotFound, "User not found in organization")
	}

	client, keycloakErr := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if keycloakErr != nil {
		return appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}

	userRoles, keycloakErr := s.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if keycloakErr != nil {
		return appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}

	rolesToDelete := slices.DeleteFunc(userRoles, func(userRole keycloak.KeycloakClientRole) bool {
		i := slices.IndexFunc(roles, func(role dto.PutUserRoleInputBody) bool {
			return role.Id == userRole.Id
		})

		return i != -1
	})

	deleteError := s.keycloakAdminClient.DeleteKeycloakUserClientRoles(client.Uuid, rolesToDelete, userId)

	if deleteError != nil {
		return appError.New(appError.InternalError, "Something went wrong", deleteError)
	}

	var keycloakRoles = []keycloak.KeycloakClientRole{}
	for _, role := range roles {
		keycloakRoles = append(keycloakRoles, keycloak.KeycloakClientRole{Id: role.Id, Name: role.Name})
	}

	keycloakErr = s.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, keycloakRoles, userId)

	if keycloakErr != nil {
		return appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}

	return nil
}

func (s *UserService) GetUserById(organizationId string, userId string) (*models.UserWithRoles, error) {

	result := policies.UserInOrganisation(s.keycloakAdminClient, organizationId, userId)

	if !result {
		return nil, appError.New(appError.EntityNotFound, "User not found in organization")
	}

	keycloakUser, keycloakErr := s.keycloakAdminClient.GetKeycloakUserById(userId)

	if keycloakErr != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}

	user := acl.FromKeycloakUser(keycloakUser)

	client, keycloakErr := s.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if keycloakErr != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}

	userRoles, keycloakErr := s.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if keycloakErr != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", keycloakErr)
	}

	roles := []models.Role{}
	for _, keycloakClientRole := range userRoles {
		roles = append(roles, acl.FromKeycloakClientRole(&keycloakClientRole))
	}

	userWithRoles := models.UserWithRoles{User: user, Roles: roles}

	return &userWithRoles, nil
}

func (s *UserService) GetUsersInformation(organizationId string) ([]models.BasicUserInformation, error) {
	user, error := s.userRepository.GetUsersInformation(organizationId)

	if error != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", error)
	}

	return user, nil
}
