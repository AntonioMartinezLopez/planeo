package setting

import "time"

type Setting struct {
	ID             int       `db:"id"`
	Host           string    `db:"host"`
	Port           int       `db:"port"`
	Username       string    `db:"username"`
	Password       string    `db:"password"`
	OrganizationID int       `db:"organization_id"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
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
