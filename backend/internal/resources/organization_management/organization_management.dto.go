package organization_management

import (
	"planeo/api/internal/clients/keycloak"
)

type KeycloakUsersOutput struct {
	Body struct {
		Users []keycloak.KeycloakUser `json:"users" doc:"Array of users managed in organization"`
	}
}

type GetKeycloakUsersInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}

type CreateKeycloakUserOutput struct {
	Body struct {
		Success bool
	}
}

type CreateKeycloakUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	Body         CreateKeycloakUserData
	RawBody      []byte
}
