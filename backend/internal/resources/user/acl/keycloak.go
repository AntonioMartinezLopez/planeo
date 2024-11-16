package acl

import (
	"planeo/api/internal/clients/keycloak"
	models "planeo/api/internal/resources/user/models"
)

func MapToKeycloakActions(actions []models.RequiredAction) []string {
	keycloakActions := []string{}

	for _, action := range actions {
		keycloakActions = append(keycloakActions, string(action))
	}

	return keycloakActions
}

func FromKeycloakActions(keycloakActions []string) []models.RequiredAction {
	actions := []models.RequiredAction{}

	for _, action := range keycloakActions {
		value, ok := models.RequiredActionsMap[action]
		if ok {
			actions = append(actions, value)
		}
	}

	return actions
}

func FromKeycloakUser(keycloakUser *keycloak.KeycloakUser) models.User {
	return models.User{
		Id:              keycloakUser.Id,
		Username:        keycloakUser.Userame,
		FirstName:       keycloakUser.FirstName,
		LastName:        keycloakUser.LastName,
		Email:           keycloakUser.Email,
		Totp:            keycloakUser.Totp,
		Enabled:         keycloakUser.Enabled,
		EmailVerified:   keycloakUser.EmailVerified,
		RequiredActions: FromKeycloakActions(keycloakUser.RequiredActions),
	}
}

func FromKeycloakClientRole(keycloakRole *keycloak.KeycloakClientRole) models.Role {
	return models.Role{
		Id:   keycloakRole.Id,
		Name: keycloakRole.Name,
	}
}
