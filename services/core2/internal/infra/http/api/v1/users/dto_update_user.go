package users

import (
	. "planeo/services/core2/internal/domain/user"
)

type UpdateUserInputBody struct {
	Username        string           `json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName       string           `json:"firstName" example:"John" doc:"First name of the user"`
	LastName        string           `json:"lastName" example:"Doe" doc:"Last name of the user"`
	Email           string           `json:"email" example:"John.Doe@planeo.de" doc:"Email of the user"`
	Totp            bool             `json:"totp" doc:"Flag describing whether TOTP was set or not"`
	Enabled         bool             `json:"enabled" doc:"Flag describing whether user is active or not"`
	EmailVerified   bool             `json:"emailVerified" doc:"Flag describing whether user email is verified or not"`
	RequiredActions []RequiredAction `json:"requiredActions" doc:"Array of actions that will be conducted after login"`
}
type UpdateUserInput struct {
	OrganizationId int    `path:"organizationId" doc:"ID of the organization"`
	Uuid           string `path:"uuid" doc:"IAM uuid of the user to be deleted"`
	Body           UpdateUserInputBody
}
