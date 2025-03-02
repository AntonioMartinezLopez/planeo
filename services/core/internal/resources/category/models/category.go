package models

import "time"

type Category struct {
	Id               int       `json:"id" db:"id"`
	Label            string    `json:"label" db:"label"`
	Color            string    `json:"color" db:"color"`
	LabelDescription string    `json:"labelDescription" db:"label_description"`
	OrganizationId   int       `json:"organizationId" db:"organization_id"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`
}
