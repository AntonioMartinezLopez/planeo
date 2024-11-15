package user

import "planeo/api/internal/clients/keycloak"

func mapToKeycloakActions(actions []RequiredAction) []string {
	keycloakActions := []string{}

	for _, action := range actions {
		keycloakActions = append(keycloakActions, string(action))
	}

	return keycloakActions
}

func fromKeycloakActions(keycloakActions []string) []RequiredAction {
	actions := []RequiredAction{}

	for _, action := range keycloakActions {
		value, ok := RequiredActionsMap[action]
		if ok {
			actions = append(actions, value)
		}
	}

	return actions
}

func fromKeycloakUser(keycloakUser *keycloak.KeycloakUser) User {
	return User{
		Id:              keycloakUser.Id,
		Username:        keycloakUser.Userame,
		FirstName:       keycloakUser.FirstName,
		LastName:        keycloakUser.LastName,
		Email:           keycloakUser.Email,
		Totp:            keycloakUser.Totp,
		Enabled:         keycloakUser.Enabled,
		EmailVerified:   keycloakUser.EmailVerified,
		RequiredActions: fromKeycloakActions(keycloakUser.RequiredActions),
	}
}

func fromKeycloakClientRole(keycloakRole *keycloak.KeycloakClientRole) Role {
	return Role{
		Id:   keycloakRole.Id,
		Name: keycloakRole.Name,
	}
}
