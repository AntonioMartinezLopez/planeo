package organization_management

import (
	"planeo/api/internal/clients/keycloak"
)

// GET users
type GetUsersOutput struct {
	Body struct {
		Users []keycloak.KeycloakUser `json:"users" doc:"Array of users managed in organization"`
	}
}

type GetUsersInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}

// GET user
type GetUserOutput struct {
	Body struct {
		User *keycloak.KeycloakUser `json:"user" doc:"Information about a user managed in keycloak"`
	}
}

type GetUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of a user"`
}

// POST user
type CreateUserOutput struct {
	Body struct {
		Success bool
	}
}

type CreateUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	Body         CreateUserData
	RawBody      []byte
}

// DELETE user
type DeleteUserOutput struct {
	CreateUserOutput
}

type DeleteUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of the user to be deleted"`
}

// PUT user/roles
type PutUserRoleInputBody struct {
	Id   string `json:"id" doc:"ID of the role to be assigned to the user"`
	Name string `json:"name" doc:"Name of the role to be assigned to the user" example:"User"`
}
type PutUserRolesInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of the user to be deleted"`
	Body         struct {
		Roles []PutUserRoleInputBody `json:"roles" doc:"Array of role representations"`
	}
}

type PutUserRoleOutput struct {
	Body struct {
		Success bool
	}
}

// GET roles
type GetRolesInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}

type GetRolesOutput struct {
	Body struct {
		Roles []keycloak.KeycloakClientRole `json:"roles" doc:"Array of roles that can be assigned to users"`
	}
}
