package keycloak

import (
	"context"
	"errors"
	"planeo/libs/logger"
	"planeo/services/core2/internal/config"
	"planeo/services/core2/internal/domain/user"
	"planeo/services/core2/pkg/keycloak"
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

func (k *KeycloakService) GetUsers(ctx context.Context, organizationUuid string) ([]user.IAMUser, error) {
	keycloakUsers, err := k.keycloakAdminClient.GetKeycloakUsers(organizationUuid)
	logger := logger.FromContext(ctx)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak users")
		return nil, err
	}

	iamUsers := []user.IAMUser{}
	for _, keycloakUser := range keycloakUsers {
		iamUsers = append(iamUsers, FromKeycloakUser(&keycloakUser))
	}

	return iamUsers, nil
}

func (k *KeycloakService) GetUserById(ctx context.Context, organizationUuid string, userId string) (*user.IAMUser, error) {
	logger := logger.FromContext(ctx)
	result := UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		logger.Error().Msg("User not found in organization")
		return nil, errors.New("User not found in organization")
	}

	keycloakUser, err := k.keycloakAdminClient.GetKeycloakUserById(userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak user")
		return nil, err
	}

	iamUser := FromKeycloakUser(keycloakUser)

	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client")
		return nil, err
	}

	userRoles, err := k.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak user roles")
		return nil, err
	}

	roles := []user.Role{}
	for _, keycloakClientRole := range userRoles {
		roles = append(roles, FromKeycloakClientRole(&keycloakClientRole))
	}

	iamUser.Roles = roles

	return &iamUser, nil

}

func (k *KeycloakService) CreateUser(ctx context.Context, organizationUuid string, newUser user.NewUser) (*user.IAMUser, error) {

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
		return nil, err
	}

	// assign default role
	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client")
		return nil, err
	}

	clientRoles, err := k.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client roles")
		return nil, err
	}

	index := slices.IndexFunc(clientRoles, func(role keycloak.KeycloakClientRole) bool {
		return role.Name == keycloak.User.String()
	})

	if index == -1 {
		logger.Error().Msg("Default role not found")
		return nil, err
	}

	role := clientRoles[index]

	keycloakUser, err := k.keycloakAdminClient.GetKeycloakUserByEmail(newUser.Email)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak user by email")
		return nil, err
	}

	err = k.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, []keycloak.KeycloakClientRole{role}, keycloakUser.Id)

	if err != nil {
		logger.Error().Err(err).Msg("Error adding keycloak client role to user")
		return nil, err
	}

	iamUser := FromKeycloakUser(keycloakUser)
	return &iamUser, nil
}

func (k *KeycloakService) UpdateUser(ctx context.Context, organizationUuid string, userId string, updateUser user.UpdateUser) error {

	logger := logger.FromContext(ctx)
	result := UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		logger.Error().Msg("User not found in organization")
		return errors.New("User not found in organization")
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
		RequiredActions: MapToKeycloakActions(updateUser.RequiredActions),
	}

	err := k.keycloakAdminClient.UpdateKeycloakUser(userId, updateKeycloakUserParams)

	if err != nil {
		logger.Error().Err(err).Msg("Error updating keycloak user")
		return err
	}

	return nil
}

func (k *KeycloakService) DeleteUser(ctx context.Context, organizationUuid string, userId string) error {

	logger := logger.FromContext(ctx)
	result := UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		logger.Error().Msg("User not found in organization")
		return errors.New("User not found in organization")
	}

	err := k.keycloakAdminClient.DeleteKeycloakUser(userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error deleting keycloak user")
		return err
	}

	return nil
}

func (k *KeycloakService) GetRoles(ctx context.Context) ([]user.Role, error) {

	logger := logger.FromContext(ctx)
	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client")
		return nil, err
	}

	keycloakClientRoles, err := k.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client roles")
		return nil, err
	}

	roles := []user.Role{}
	for _, keycloakClientRole := range keycloakClientRoles {
		roles = append(roles, FromKeycloakClientRole(&keycloakClientRole))
	}

	return roles, nil
}

func (k *KeycloakService) AssignRolesToUser(ctx context.Context, organizationUuid string, userId string, roles []user.Role) error {
	logger := logger.FromContext(ctx)
	result := UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		logger.Error().Msg("User not found in organization")
		return errors.New("User not found in organization")
	}

	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak client")
		return err
	}

	userRoles, err := k.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error getting keycloak user roles")
		return err
	}

	rolesToDelete := slices.DeleteFunc(userRoles, func(userRole keycloak.KeycloakClientRole) bool {
		i := slices.IndexFunc(roles, func(role user.Role) bool {
			return role.Id == userRole.Id
		})

		return i != -1
	})

	deleteError := k.keycloakAdminClient.DeleteKeycloakUserClientRoles(client.Uuid, rolesToDelete, userId)

	if deleteError != nil {
		logger.Error().Err(deleteError).Msg("Error deleting keycloak user roles")
		return err
	}

	var keycloakRoles = []keycloak.KeycloakClientRole{}
	for _, role := range roles {
		keycloakRoles = append(keycloakRoles, keycloak.KeycloakClientRole{Id: role.Id, Name: role.Name})
	}

	err = k.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, keycloakRoles, userId)

	if err != nil {
		logger.Error().Err(err).Msg("Error adding keycloak client role to user")
		return err
	}

	return nil
}
