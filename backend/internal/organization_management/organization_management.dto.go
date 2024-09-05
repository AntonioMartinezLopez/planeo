package organization_management

import (
	keycloak "planeo/api/internal/organization_management/keycloak_queries"
)

type KeycloakUserOutput struct {
	Body struct {
		Users []keycloak.KeycloakUser `json:"users" doc:"Array of users managed in organization"`
	}
}

type GetKeycloakUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}
