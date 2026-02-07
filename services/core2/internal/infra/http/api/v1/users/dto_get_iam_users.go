package users

import (
	. "planeo/services/core2/internal/domain/user"
)

// GET users
type GetIAMUsersOutput struct {
	Body struct {
		Users []IAMUser `json:"users" doc:"Array of users managed in organization"`
	}
}

type GetIAMUsersInput struct {
	OrganizationId int  `path:"organizationId" doc:"ID of the organization"`
	Sync           bool `query:"sync" required:"false" doc:"Flag describing whether to sync users from auth system or not"`
}
