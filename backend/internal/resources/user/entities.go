package user

import (
	"github.com/danielgtaylor/huma/v2"
)

type RequiredAction string

const (
	ActionConfigureTotp  RequiredAction = "CONFIGURE_TOTP"
	ActionUpdatePassword RequiredAction = "UPDATE_PASSWORD"
	ActionUpdateProfile  RequiredAction = "UPDATE_PROFILE"
	ActionVerifyEmail    RequiredAction = "VERIFY_EMAIL"
)

var RequiredActionsMap = map[string]RequiredAction{
	"CONFIGURE_TOTP":  ActionConfigureTotp,
	"UPDATE_PASSWORD": ActionUpdatePassword,
	"UPDATE_PROFILE":  ActionUpdateProfile,
	"VERIFY_EMAIL":    ActionVerifyEmail,
}

var _ huma.SchemaTransformer = RequiredAction("")

func (ra RequiredAction) TransformSchema(r huma.Registry, s *huma.Schema) *huma.Schema {
	s.Enum = []interface{}{"CONFIGURE_TOTP", "UPDATE_PASSWORD", "UPDATE_PROFILE", "VERIFY_EMAIL"}
	s.Default = ActionConfigureTotp
	s.PrecomputeMessages()
	return s
}

type User struct {
	Id              string           `json:"id" example:"123456" doc:"User identifier within the authentication system" validate:"required"`
	Userame         string           `json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName       string           `json:"firstName" example:"John" doc:"First name of the user" validate:"required"`
	LastName        string           `json:"lastName" validate:"required" example:"Doe" doc:"Last name of the user"`
	Email           string           `json:"email" validate:"required" example:"John.Doe@planeo.de" doc:"Email of the user"`
	Totp            bool             `json:"totp" doc:"Flag describing whether TOTP was set or not"`
	Enabled         bool             `json:"enabled" doc:"Flag describing whether user is active or not"`
	EmailVerified   bool             `json:"emailVerified" doc:"Flag describing whether user email is verified or not"`
	RequiredActions []RequiredAction `json:"requiredActions" validate:"required" doc:"Array of actions that will be conducted after login"`
}

type Role struct {
	Id   string `json:"id" doc:"ID of the role to be assigned to the user"`
	Name string `json:"name" doc:"Name of the role to be assigned to the user" example:"User"`
}

type UserWithRoles struct {
	User
	Roles []Role
}
