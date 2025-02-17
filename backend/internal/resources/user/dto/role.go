package dto

import models "planeo/api/internal/resources/user/models"

// GET roles
type GetRolesInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
}

type GetRolesOutput struct {
	Body struct {
		Roles []models.Role `json:"roles" doc:"Array of roles that can be assigned to users"`
	}
}

// PUT user/roles
type PutUserRoleInputBody struct {
	models.Role
}
type PutUserRolesInput struct {
	Organization string                 `path:"organization" doc:"ID of the organization"`
	UserId       string                 `path:"userId" doc:"ID of the user to be deleted"`
	Body         []PutUserRoleInputBody `doc:"Array of role representations"`
}

type PutUserRoleOutput struct {
	Body struct {
		Success bool
	}
}
