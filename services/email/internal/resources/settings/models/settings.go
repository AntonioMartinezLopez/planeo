package models

import (
	"time"
)

type Setting struct {
	ID             int       `json:"id" pgx:"id"`
	Host           string    `json:"host" pgx:"host"`
	Port           int       `json:"port" pgx:"port"`
	Username       string    `json:"username" pgx:"username"`
	Password       string    `json:"password" pgx:"password"`
	OrganizationID int       `json:"organization_id" pgx:"organization_id"`
	CreatedAt      time.Time `json:"created_at" pgx:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" pgx:"updated_at"`
}
