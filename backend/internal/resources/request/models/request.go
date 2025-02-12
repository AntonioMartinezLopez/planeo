package models

import "time"

type Request struct {
	Id             int       `db:"id" json:"id"`
	Text           string    `db:"text" json:"text" example:"I would like to renovate my bathroom" doc:"Description of the request"`
	Name           string    `db:"name" json:"name" example:"John Doe" doc:"name of the requester"`
	Email          string    `db:"email" json:"email" example:"John.Doe@example.com" doc:"Email of the requester"`
	Address        string    `db:"address" json:"address" example:"123 Main St" doc:"Address of the requester"`
	Telephone      string    `db:"telephone" json:"telephone" example:"123-456-7890" doc:"Telephone number of the requester"`
	Closed         bool      `db:"closed" json:"closed" example:"false" doc:"Indicates if the request is closed"`
	CategoryId     int       `db:"category_id" json:"categoryId" doc:"Identifier of the category"`
	OrganizationId int       `db:"organization_id" json:"organizationId" doc:"Identifier of the organization"`
	CreatedAt      time.Time `db:"created_at" json:"createdAt" doc:"Timestamp when the request was created"`
	UpdatedAt      time.Time `db:"updated_at" json:"updatedAt" doc:"Timestamp when the request was last updated"`
}
