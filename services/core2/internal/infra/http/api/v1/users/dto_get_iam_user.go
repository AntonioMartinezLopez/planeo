package users

import (
	. "planeo/services/core2/internal/domain/user"
)

// GET user
type GetIAMUserOutput struct {
	Body struct {
		User *IAMUser `json:"user" doc:"Information about a user managed in given auth system"`
	}
}

type GetIAMUserInput struct {
	OrganizationId int    `path:"organizationId" doc:"ID of the organization"`
	Uuid           string `path:"uuid" doc:"IAM uuid of a user"`
}
