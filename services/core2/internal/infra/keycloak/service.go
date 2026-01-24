package keycloak

import (
	"context"
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

	if err != nil {
		return nil, NewKeycloakError("Error getting keycloak users", err)
	}

	iamUsers := []user.IAMUser{}
	for _, keycloakUser := range keycloakUsers {
		iamUsers = append(iamUsers, FromKeycloakUser(&keycloakUser))
	}

	return iamUsers, nil
}

func (k *KeycloakService) GetUserById(ctx context.Context, organizationUuid string, userId string) (*user.IAMUser, error) {
	result := UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		return nil, NewKeycloakError("User not found in organization", nil)
	}

	keycloakUser, err := k.keycloakAdminClient.GetKeycloakUserById(userId)

	if err != nil {
		return nil, NewKeycloakError("Error getting keycloak user by id", err)
	}

	iamUser := FromKeycloakUser(keycloakUser)

	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		return nil, NewKeycloakError("Error getting keycloak client", err)
	}

	userRoles, err := k.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		return nil, NewKeycloakError("Error getting keycloak user roles", err)
	}

	roles := []user.Role{}
	for _, keycloakClientRole := range userRoles {
		roles = append(roles, FromKeycloakClientRole(&keycloakClientRole))
	}

	iamUser.Roles = roles

	return &iamUser, nil

}

func (k *KeycloakService) CreateUser(ctx context.Context, organizationUuid string, newUser user.NewUser) (*user.IAMUser, error) {
	createUserData := keycloak.CreateUserParams{
		FirstName: newUser.FirstName,
		LastName:  newUser.LastName,
		Email:     newUser.Email,
		Password:  newUser.Password,
	}
	err := k.keycloakAdminClient.CreateKeycloakUser(organizationUuid, createUserData)

	if err != nil {
		return nil, NewKeycloakError("Error creating keycloak user", err)
	}

	// assign default role
	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		return nil, NewKeycloakError("Error getting keycloak client", err)
	}

	clientRoles, err := k.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		return nil, NewKeycloakError("Error getting keycloak client roles", err)
	}

	index := slices.IndexFunc(clientRoles, func(role keycloak.KeycloakClientRole) bool {
		return role.Name == keycloak.User.String()
	})

	if index == -1 {
		return nil, NewKeycloakError("Default user role not found", nil)
	}

	role := clientRoles[index]

	keycloakUser, err := k.keycloakAdminClient.GetKeycloakUserByEmail(newUser.Email)

	if err != nil {
		return nil, NewKeycloakError("Error getting keycloak user by email", err)
	}

	err = k.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, []keycloak.KeycloakClientRole{role}, keycloakUser.Id)

	if err != nil {
		return nil, NewKeycloakError("Error assigning default role to user", err)
	}

	iamUser := FromKeycloakUser(keycloakUser)
	return &iamUser, nil
}

func (k *KeycloakService) UpdateUser(ctx context.Context, organizationUuid string, userId string, updateUser user.UpdateUser) error {
	result := UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		return NewKeycloakError("User not found in organization", nil)
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
		return NewKeycloakError("Error updating keycloak user", err)
	}

	return nil
}

func (k *KeycloakService) DeleteUser(ctx context.Context, organizationUuid string, userId string) error {
	result := UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		return NewKeycloakError("User not found in organization", nil)
	}

	err := k.keycloakAdminClient.DeleteKeycloakUser(userId)

	if err != nil {
		return NewKeycloakError("Error deleting keycloak user", err)
	}

	return nil
}

func (k *KeycloakService) GetRoles(ctx context.Context) ([]user.Role, error) {
	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		return nil, NewKeycloakError("Error getting keycloak client", err)
	}

	keycloakClientRoles, err := k.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		return nil, NewKeycloakError("Error getting keycloak client roles", err)
	}

	roles := []user.Role{}
	for _, keycloakClientRole := range keycloakClientRoles {
		roles = append(roles, FromKeycloakClientRole(&keycloakClientRole))
	}

	return roles, nil
}

func (k *KeycloakService) AssignRolesToUser(ctx context.Context, organizationUuid string, userId string, roles []user.Role) error {
	result := UserInOrganization(k.keycloakAdminClient, organizationUuid, userId)

	if !result {
		return NewKeycloakError("User not found in organization", nil)
	}

	client, err := k.keycloakAdminClient.GetKeycloakClient(k.config.KcOauthClientID)

	if err != nil {
		return NewKeycloakError("Error getting keycloak client", err)
	}

	userRoles, err := k.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		return NewKeycloakError("Error getting keycloak user client roles", err)
	}

	rolesToDelete := slices.DeleteFunc(userRoles, func(userRole keycloak.KeycloakClientRole) bool {
		i := slices.IndexFunc(roles, func(role user.Role) bool {
			return role.Id == userRole.Id
		})

		return i != -1
	})

	deleteError := k.keycloakAdminClient.DeleteKeycloakUserClientRoles(client.Uuid, rolesToDelete, userId)

	if deleteError != nil {
		return NewKeycloakError("Error deleting keycloak user client roles", deleteError)
	}

	var keycloakRoles = []keycloak.KeycloakClientRole{}
	for _, role := range roles {
		keycloakRoles = append(keycloakRoles, keycloak.KeycloakClientRole{Id: role.Id, Name: role.Name})
	}

	err = k.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, keycloakRoles, userId)

	if err != nil {
		return NewKeycloakError("Error adding keycloak client roles to user", err)
	}

	return nil
}
