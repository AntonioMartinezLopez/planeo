package user

import (
	"context"
	appError "planeo/libs/errors"
	"planeo/libs/logger"
	"planeo/services/core/config"
	"planeo/services/core/internal/clients/keycloak"
	"planeo/services/core/internal/resources/user/acl"
	"planeo/services/core/internal/resources/user/models"
	"planeo/services/core/internal/resources/user/policies"
	"slices"
)

type KeycloakService struct {
	keycloakAdminClient *keycloak.KeycloakAdminClient
	config              *config.ApplicationConfiguration
}

func NewKeycloakService(keycloakAdminClient *keycloak.KeycloakAdminClient, config *config.ApplicationConfiguration) *KeycloakService {
	return &KeycloakService{
		keycloakAdminClient: keycloakAdminClient,
		config:              config,
	}
}

func (k *KeycloakService) GetUsers(ctx context.Context, organizationUuid string) ([]models.User, error) {
	keycloakUsers, err := k.keycloakAdminClient.GetKeycloakUsers(organizationUuid)
	logger := logger.FromContext(ctx)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak users")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	users := []models.User{}
	for _, keycloakUser := range keycloakUsers {
		users = append(users, acl.FromKeycloakUser(&keycloakUser))
	}

	return users, nil
}

func (k *KeycloakService) GetUserById(ctx context.Context, organizationUuid string, userId string) (*models.User, error) {
	logger := logger.FromContext(ctx)
	result := policies.UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		logger.Error().Msg("User not found in organization")
		return nil, appError.New(appError.EntityNotFound, "User not found in organization")
	}

	keycloakUser, err := k.keycloakAdminClient.GetKeycloakUserById(userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak user")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	user := acl.FromKeycloakUser(keycloakUser)

	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	userRoles, err := k.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak user roles")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	roles := []models.Role{}
	for _, keycloakClientRole := range userRoles {
		roles = append(roles, acl.FromKeycloakClientRole(&keycloakClientRole))
	}

	user.Roles = roles

	return &user, nil

}

func (k *KeycloakService) CreateUser(ctx context.Context, organizationUuid string, newUser models.NewUser) (*models.User, error) {

	logger := logger.FromContext(ctx)
	createUserData := keycloak.CreateUserParams{
		FirstName: newUser.FirstName,
		LastName:  newUser.LastName,
		Email:     newUser.Email,
		Password:  newUser.Password,
	}
	err := k.keycloakAdminClient.CreateKeycloakUser(organizationUuid, createUserData)

	if err != nil {
		logger.Error().Err(err).Msg("Error creating keycloak user")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	// assign default role
	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	clientRoles, err := k.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client roles")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	index := slices.IndexFunc(clientRoles, func(role keycloak.KeycloakClientRole) bool {
		return role.Name == keycloak.User.String()
	})

	if index == -1 {
		logger.Error().Msg("Default role not found")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	role := clientRoles[index]

	user, err := k.keycloakAdminClient.GetKeycloakUserByEmail(newUser.Email)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak user by email")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	err = k.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, []keycloak.KeycloakClientRole{role}, user.Id)

	if err != nil {
		logger.Error().Err(err).Msg("Error adding keycloak client role to user")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	userModel := acl.FromKeycloakUser(user)
	return &userModel, nil
}

func (k *KeycloakService) UpdateUser(ctx context.Context, organizationUuid string, userId string, updateUser models.UpdateUser) error {

	logger := logger.FromContext(ctx)
	result := policies.UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		logger.Error().Msg("User not found in organization")
		return appError.New(appError.EntityNotFound, "User not found in organization")
	}

	updateKeycloakUserParams := keycloak.UpdateUserParams{
		Id:              userId,
		Userame:         updateUser.Username,
		FirstName:       updateUser.FirstName,
		LastName:        updateUser.LastName,
		Email:           updateUser.Email,
		Enabled:         updateUser.Enabled,
		EmailVerified:   updateUser.EmailVerified,
		Totp:            updateUser.Totp,
		RequiredActions: acl.MapToKeycloakActions(updateUser.RequiredActions),
	}

	err := k.keycloakAdminClient.UpdateKeycloakUser(userId, updateKeycloakUserParams)

	if err != nil {
		logger.Error().Err(err).Msg("Error updating keycloak user")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (k *KeycloakService) DeleteUser(ctx context.Context, organizationUuid string, userId string) error {

	logger := logger.FromContext(ctx)
	result := policies.UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		logger.Error().Msg("User not found in organization")
		return appError.New(appError.EntityNotFound, "User not found in organization")
	}

	err := k.keycloakAdminClient.DeleteKeycloakUser(userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error deleting keycloak user")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (k *KeycloakService) GetRoles(ctx context.Context) ([]models.Role, error) {

	logger := logger.FromContext(ctx)
	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	keycloakClientRoles, err := k.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client roles")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	roles := []models.Role{}
	for _, keycloakClientRole := range keycloakClientRoles {
		roles = append(roles, acl.FromKeycloakClientRole(&keycloakClientRole))
	}

	return roles, nil
}

func (k *KeycloakService) AssignRolesToUser(ctx context.Context, organizationUuid string, userId string, roles []models.Role) error {
	logger := logger.FromContext(ctx)
	result := policies.UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		logger.Error().Msg("User not found in organization")
		return appError.New(appError.EntityNotFound, "User not found in organization")
	}

	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	userRoles, err := k.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak user roles")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	rolesToDelete := slices.DeleteFunc(userRoles, func(userRole keycloak.KeycloakClientRole) bool {
		i := slices.IndexFunc(roles, func(role models.Role) bool {
			return role.Id == userRole.Id
		})

		return i != -1
	})

	deleteError := k.keycloakAdminClient.DeleteKeycloakUserClientRoles(client.Uuid, rolesToDelete, userId)

	if deleteError != nil {
		logger.Error().Err(deleteError).Msg("Error deleting keycloak user roles")
		return appError.New(appError.InternalError, "Something went wrong", deleteError)
	}

	var keycloakRoles = []keycloak.KeycloakClientRole{}
	for _, role := range roles {
		keycloakRoles = append(keycloakRoles, keycloak.KeycloakClientRole{Id: role.Id, Name: role.Name})
	}

	err = k.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, keycloakRoles, userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error adding keycloak client role to user")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}
