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

type KeycloakService struct {
	keycloakAdminClient *keycloak.KeycloakAdminClient
}

func NewKeycloakService(keycloakAdminClient *keycloak.KeycloakAdminClient) *KeycloakService {
	return &KeycloakService{
		keycloakAdminClient: keycloakAdminClient,
	}
}

func (k *KeycloakService) GetUsers(organizationId string) ([]models.User, error) {
	keycloakUsers, err := k.keycloakAdminClient.GetKeycloakUsers(organizationId)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	users := []models.User{}
	for _, keycloakUser := range keycloakUsers {
		users = append(users, acl.FromKeycloakUser(&keycloakUser))
	}

	return users, nil
}

func (k *KeycloakService) GetUserById(organizationId string, userId string) (*models.UserWithRoles, error) {

	result := policies.UserInOrganisation(k.keycloakAdminClient, organizationId, userId)

	if !result {
		return nil, appError.New(appError.EntityNotFound, "User not found in organization")
	}

	keycloakUser, err := k.keycloakAdminClient.GetKeycloakUserById(userId)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	user := acl.FromKeycloakUser(keycloakUser)

	client, err := k.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	userRoles, err := k.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	roles := []models.Role{}
	for _, keycloakClientRole := range userRoles {
		roles = append(roles, acl.FromKeycloakClientRole(&keycloakClientRole))
	}

	userWithRoles := models.UserWithRoles{User: user, Roles: roles}

	return &userWithRoles, nil

}

func (k *KeycloakService) CreateUser(organizationId string, createUserInput dto.CreateUserInputBody) (*models.User, error) {

	createUserData := keycloak.CreateUserParams{
		FirstName: createUserInput.FirstName,
		LastName:  createUserInput.LastName,
		Email:     createUserInput.Email,
		Password:  createUserInput.Password,
	}
	err := k.keycloakAdminClient.CreateKeycloakUser(organizationId, createUserData)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	// assign default role
	client, err := k.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	clientRoles, err := k.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	index := slices.IndexFunc(clientRoles, func(role keycloak.KeycloakClientRole) bool {
		return role.Name == keycloak.User.String()
	})

	if index == -1 {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	role := clientRoles[index]

	user, err := k.keycloakAdminClient.GetKeycloakUserByEmail(createUserData.Email)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	err = k.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, []keycloak.KeycloakClientRole{role}, user.Id)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	userModel := acl.FromKeycloakUser(user)
	return &userModel, nil
}

func (k *KeycloakService) UpdateUser(organizationId string, userId string, updateUserInput dto.UpdateUserInputBody) error {

	result := policies.UserInOrganisation(k.keycloakAdminClient, organizationId, userId)

	if !result {
		return appError.New(appError.EntityNotFound, "User not found in organization")
	}

	updateKeycloakUserParams := keycloak.UpdateUserParams{
		Id:              userId,
		Userame:         updateUserInput.Username,
		FirstName:       updateUserInput.FirstName,
		LastName:        updateUserInput.LastName,
		Email:           updateUserInput.Email,
		Enabled:         updateUserInput.Enabled,
		EmailVerified:   updateUserInput.EmailVerified,
		Totp:            updateUserInput.Totp,
		RequiredActions: acl.MapToKeycloakActions(updateUserInput.RequiredActions),
	}

	err := k.keycloakAdminClient.UpdateKeycloakUser(userId, updateKeycloakUserParams)

	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (k *KeycloakService) DeleteUser(organizationId string, userId string) error {

	result := policies.UserInOrganisation(k.keycloakAdminClient, organizationId, userId)

	if !result {
		return appError.New(appError.EntityNotFound, "User not found in organization")
	}

	err := k.keycloakAdminClient.DeleteKeycloakUser(userId)

	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (k *KeycloakService) GetRoles() ([]models.Role, error) {

	client, err := k.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	keycloakClientRoles, err := k.keycloakAdminClient.GetKeycloakClientRoles(client.Uuid)

	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	roles := []models.Role{}
	for _, keycloakClientRole := range keycloakClientRoles {
		roles = append(roles, acl.FromKeycloakClientRole(&keycloakClientRole))
	}

	return roles, nil
}

func (k *KeycloakService) AssignRolesToUser(organizationId string, userId string, roles []dto.PutUserRoleInputBody) error {
	result := policies.UserInOrganisation(k.keycloakAdminClient, organizationId, userId)

	if !result {
		return appError.New(appError.EntityNotFound, "User not found in organization")
	}

	client, err := k.keycloakAdminClient.GetKeycloakClient(config.Config.KcOauthClientID)

	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	userRoles, err := k.keycloakAdminClient.GetKeycloakUserClientRoles(client.Uuid, userId)

	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	rolesToDelete := slices.DeleteFunc(userRoles, func(userRole keycloak.KeycloakClientRole) bool {
		i := slices.IndexFunc(roles, func(role dto.PutUserRoleInputBody) bool {
			return role.Id == userRole.Id
		})

		return i != -1
	})

	deleteError := k.keycloakAdminClient.DeleteKeycloakUserClientRoles(client.Uuid, rolesToDelete, userId)

	if deleteError != nil {
		return appError.New(appError.InternalError, "Something went wrong", deleteError)
	}

	var keycloakRoles = []keycloak.KeycloakClientRole{}
	for _, role := range roles {
		keycloakRoles = append(keycloakRoles, keycloak.KeycloakClientRole{Id: role.Id, Name: role.Name})
	}

	err = k.keycloakAdminClient.AddKeycloakClientRoleToUser(client.Uuid, keycloakRoles, userId)

	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}
