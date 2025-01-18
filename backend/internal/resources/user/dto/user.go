package dto

import (
	models "planeo/api/internal/resources/user/models"
)

// GET users
type GetUsersOutput struct {
	Body struct {
		Users []models.User `json:"users" doc:"Array of users managed in organization"`
	}
}

type GetUsersInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	Sync         bool   `query:"sync" required:"false" doc:"Flag describing whether to sync users from auth system or not"`
}

// GET user
type GetUserOutput struct {
	Body struct {
		User *models.User `json:"user" doc:"Information about a user managed in given auth system"`
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

type CreateUserInputBody struct {
	FirstName string `json:"firstName" doc:"First name of the user to be created" example:"John"`
	LastName  string `json:"lastName" doc:"Last name of the user to be created" example:"Doe"`
	Email     string `json:"email" doc:"Email of the user to be created" example:"John.Doe@planeo.de"`
	Password  string `json:"password" doc:"Initial password for the user to be set" example:"password123"`
}

type CreateUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	Body         CreateUserInputBody
	RawBody      []byte
}

// UPDATE user
type UpdateUserInputBody struct {
	Username        string                  `json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName       string                  `json:"firstName" example:"John" doc:"First name of the user" validate:"required"`
	LastName        string                  `json:"lastName" validate:"required" example:"Doe" doc:"Last name of the user"`
	Email           string                  `json:"email" validate:"required" example:"John.Doe@planeo.de" doc:"Email of the user"`
	Totp            bool                    `json:"totp" doc:"Flag describing whether TOTP was set or not"`
	Enabled         bool                    `json:"enabled" doc:"Flag describing whether user is active or not"`
	EmailVerified   bool                    `json:"emailVerified" doc:"Flag describing whether user email is verified or not"`
	RequiredActions []models.RequiredAction `json:"requiredActions" validate:"required" doc:"Array of actions that will be conducted after login"`
}
type UpdateUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of the user to be deleted"`
	Body         UpdateUserInputBody
}

type UpdateUserOutput struct {
	CreateUserOutput
}

// DELETE user
type DeleteUserOutput struct {
	CreateUserOutput
}

type DeleteUserInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	UserId       string `path:"userId" doc:"ID of the user to be deleted"`
}
