package organization_management

import (
	"planeo/api/internal/clients/keycloak"
)

// GET users
type KeycloakUsersOutput struct {
	Body struct {
		Users []keycloak.KeycloakUser `json:"users" doc:"Array of users managed in organization"`
	}
}

type GetKeycloakUsersInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}

// GET user
type KeycloakUserOutput struct {
	Body struct {
		User *keycloak.KeycloakUser `json:"user" doc:"Information about a user managed in keycloak"`
	}
}

type GetKeycloakUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of a user"`
}

// POST user
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

// DELETE USER
type DeleteKeycloakUserOutput struct {
	CreateKeycloakUserOutput
}

type DeleteKeycloakUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of the user to be deleted"`
}

// GET roles
type GetKeycloakRolesInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}

type GetKeycloakRolesOutput struct {
	Body struct {
		Roles []keycloak.KeycloakClientRole `json:"roles" doc:"Array of roles that can be assigned to users"`
	}
}
