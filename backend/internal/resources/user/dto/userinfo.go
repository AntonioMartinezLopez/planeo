package dto

import models "planeo/api/internal/resources/user/models"

// GET userinfo
type GetUserInfoInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
}

type GetUserInfoOutput struct {
	Body struct {
		Users []models.BasicUserInformation `json:"users" doc:"Array of users with basic informations"`
	}
}
