package models

import (
	"time"
)

type Setting struct {
	ID             int       `pgx:"id"`
	Host           string    `pgx:"host"`
	Port           int       `pgx:"port"`
	Username       string    `pgx:"username"`
	Password       string    `pgx:"password"`
	OrganizationID int       `pgx:"organization_id"`
	CreatedAt      time.Time `pgx:"created_at"`
	UpdatedAt      time.Time `pgx:"updated_at"`
}

type NewSetting struct {
	Host           string
	Port           int
	Username       string
	Password       string
	OrganizationID int
}

type UpdateSetting struct {
	ID             int
	Host           string
	Port           int
	Username       string
	Password       string
	OrganizationID int
}
