package user

import "time"

type Role struct {
	Id   string `json:"id" doc:"ID of the role to be assigned to the user"`
	Name string `json:"name" doc:"Name of the role to be assigned to the user" example:"User"`
}

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

// var _ huma.SchemaTransformer = RequiredAction("")

// func (ra RequiredAction) TransformSchema(r huma.Registry, s *huma.Schema) *huma.Schema {
// 	s.Enum = []interface{}{"CONFIGURE_TOTP", "UPDATE_PASSWORD", "UPDATE_PROFILE", "VERIFY_EMAIL"}
// 	s.Default = ActionConfigureTotp
// 	s.PrecomputeMessages()
// 	return s
// }

type IAMUser struct {
	Uuid            string           `json:"uuid" example:"123456" doc:"User identifier within the authentication system" validate:"required"`
	Username        string           `json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName       string           `json:"firstName" example:"John" doc:"First name of the user" validate:"required"`
	LastName        string           `json:"lastName" validate:"required" example:"Doe" doc:"Last name of the user"`
	Email           string           `json:"email" validate:"required" example:"John.Doe@planeo.de" doc:"Email of the user"`
	Totp            bool             `json:"totp" doc:"Flag describing whether TOTP was set or not"`
	Enabled         bool             `json:"enabled" doc:"Flag describing whether user is active or not"`
	EmailVerified   bool             `json:"emailVerified" doc:"Flag describing whether user email is verified or not"`
	RequiredActions []RequiredAction `json:"requiredActions" validate:"required" doc:"Array of actions that will be conducted after login"`
	Roles           []Role           `json:"roles,omitempty" doc:"Array of roles assigned to the user"`
}

type User struct {
	Id             int       `db:"id" json:"id"`
	Username       string    `db:"username" json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName      string    `db:"first_name" json:"firstName" example:"John" doc:"First name of the user" validate:"required"`
	LastName       string    `db:"last_name" json:"lastName" validate:"required" example:"Doe" doc:"Last name of the user"`
	Email          string    `db:"email" json:"email" validate:"required" example:"John.Doe@planeo.de" doc:"Email of the user"`
	UUID           string    `db:"uuid" json:"uuid" doc:"UUID of the user in the IAM system"`
	OrganizationId int       `db:"organization_id" json:"organization"`
	CreatedAt      time.Time `db:"created_at" json:"createdAt" doc:"Timestamp when the user was created"`
	UpdatedAt      time.Time `db:"updated_at" json:"updatedAt" doc:"Timestamp when the user was last updated"`
}

type NewUser struct {
	Username  string
	FirstName string
	LastName  string
	Email     string
	Password  string
}

type UpdateUser struct {
	Username        string
	FirstName       string
	LastName        string
	Email           string
	Totp            bool
	Enabled         bool
	EmailVerified   bool
	RequiredActions []RequiredAction
}
