package users

import (
	. "planeo/services/core2/internal/domain/user"
)

// PUT user/roles
type PutUserRoleInputBody struct {
	Role
}
type PutUserRolesInput struct {
	OrganizationId int                    `path:"organizationId" doc:"ID of the organization"`
	Uuid           string                 `path:"uuid" doc:"ID of the user to be deleted"`
	Body           []PutUserRoleInputBody `doc:"Array of role representations"`
}
