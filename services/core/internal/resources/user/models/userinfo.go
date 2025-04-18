package models

import "time"

type BasicUserInformation struct {
	Id             int       `db:"id" json:"id"`
	Username       string    `db:"username" json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName      string    `db:"first_name" json:"firstName" example:"John" doc:"First name of the user" validate:"required"`
	LastName       string    `db:"last_name" json:"lastName" validate:"required" example:"Doe" doc:"Last name of the user"`
	Email          string    `db:"email" json:"email" validate:"required" example:"John.Doe@planeo.de" doc:"Email of the user"`
	IAMUserID      string    `db:"iam_user_id" json:"iamUserId"`
	OrganizationId int       `db:"organization_id" json:"organization"`
	CreatedAt      time.Time `db:"created_at" json:"createdAt" doc:"Timestamp when the user was created"`
	UpdatedAt      time.Time `db:"updated_at" json:"updatedAt" doc:"Timestamp when the user was last updated"`
}
