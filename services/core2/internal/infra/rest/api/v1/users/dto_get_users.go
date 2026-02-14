package users

import . "planeo/services/core2/internal/domain/user"

type GetUsersInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
}

type GetUsersOutput struct {
	Body struct {
		Users []User `json:"users" doc:"Array of users with basic informations"`
	}
}
