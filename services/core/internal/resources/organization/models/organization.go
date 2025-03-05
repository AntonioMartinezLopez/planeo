package models

import (
	"time"
)

type Organization struct {
	Id                int       `json:"id" db:"id"`
	Name              string    `json:"name" db:"name"`
	Address           string    `json:"address" db:"address"`
	Email             string    `json:"email" db:"email"`
	IAMOrganizationID string    `json:"iam_organization_id" db:"iam_organization_id"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}
