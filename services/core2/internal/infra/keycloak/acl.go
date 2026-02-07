package keycloak

import (
	"planeo/services/core2/internal/domain/user"
	"planeo/services/core2/pkg/keycloak"
)

func MapToKeycloakActions(actions []user.RequiredAction) []string {
	keycloakActions := []string{}

	for _, action := range actions {
		keycloakActions = append(keycloakActions, string(action))
	}

	return keycloakActions
}

func FromKeycloakActions(keycloakActions []string) []user.RequiredAction {
	actions := []user.RequiredAction{}

	for _, action := range keycloakActions {
		value, ok := user.RequiredActionsMap[action]
		if ok {
			actions = append(actions, value)
		}
	}

	return actions
}

func FromKeycloakUser(keycloakUser *keycloak.KeycloakUser) user.IAMUser {
	return user.IAMUser{
		Uuid:            keycloakUser.Id,
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

func FromKeycloakClientRole(keycloakRole *keycloak.KeycloakClientRole) user.Role {
	return user.Role{
		Id:   keycloakRole.Id,
		Name: keycloakRole.Name,
	}
}
