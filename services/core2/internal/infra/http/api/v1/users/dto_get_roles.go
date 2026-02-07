package users

import (
	. "planeo/services/core2/internal/domain/user"
)

// GET roles
type GetRolesInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
}

type GetRolesOutput struct {
	Body struct {
		Roles []Role `json:"roles" doc:"Array of roles that can be assigned to users"`
	}
}
