package models

type BasicUserInformation struct {
	ID           string `db:"id" json:"id"`
	Username     string `db:"username" json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName    string `db:"first_name" json:"firstName" example:"John" doc:"First name of the user" validate:"required"`
	LastName     string `db:"last_name" json:"lastName" validate:"required" example:"Doe" doc:"Last name of the user"`
	Email        string `db:"email" json:"email" validate:"required" example:"John.Doe@planeo.de" doc:"Email of the user"`
	KeycloakId   string `db:"keycloak_id" json:"keycloakId"`
	Organization string `db:"organization" json:"organization"`
}
