package dto

import (
	models "planeo/services/core/internal/resources/user/models"
)

// GET users
type GetUsersOutput struct {
	Body struct {
		Users []models.User `json:"users" doc:"Array of users managed in organization"`
	}
}

type GetUsersInput struct {
	OrganizationId int  `path:"organizationId" doc:"ID of the organization"`
	Sync           bool `query:"sync" required:"false" doc:"Flag describing whether to sync users from auth system or not"`
}

// GET user
type GetUserOutput struct {
	Body struct {
		User *models.User `json:"user" doc:"Information about a user managed in given auth system"`
	}
}

type GetUserInput struct {
	OrganizationId int    `path:"organizationId" doc:"ID of the organization"`
	IamUserId      string `path:"iamUserId" doc:"IAM id of a user"`
}

// POST user
type CreateUserInputBody struct {
	FirstName string `json:"firstName" doc:"First name of the user to be created" example:"John"`
	LastName  string `json:"lastName" doc:"Last name of the user to be created" example:"Doe"`
	Email     string `json:"email" doc:"Email of the user to be created" example:"John.Doe@planeo.de"`
	Password  string `json:"password" doc:"Initial password for the user to be set" example:"password123"`
}

type CreateUserInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	Body           CreateUserInputBody
	RawBody        []byte
}

// UPDATE user
type UpdateUserInputBody struct {
	Username        string                  `json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName       string                  `json:"firstName" example:"John" doc:"First name of the user"`
	LastName        string                  `json:"lastName" example:"Doe" doc:"Last name of the user"`
	Email           string                  `json:"email" example:"John.Doe@planeo.de" doc:"Email of the user"`
	Totp            bool                    `json:"totp" doc:"Flag describing whether TOTP was set or not"`
	Enabled         bool                    `json:"enabled" doc:"Flag describing whether user is active or not"`
	EmailVerified   bool                    `json:"emailVerified" doc:"Flag describing whether user email is verified or not"`
	RequiredActions []models.RequiredAction `json:"requiredActions" doc:"Array of actions that will be conducted after login"`
}
type UpdateUserInput struct {
	OrganizationId int    `path:"organizationId" doc:"ID of the organization"`
	IamUserId      string `path:"iamUserId" doc:"IAM id of the user to be deleted"`
	Body           UpdateUserInputBody
}

// DELETE user
type DeleteUserInput struct {
	OrganizationId int    `path:"organizationId" doc:"ID of the organization"`
	IamUserId      string `path:"iamUserId" doc:"IAM id of the user to be deleted"`
}
