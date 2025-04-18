package models

import "time"

type Category struct {
	Id               int       `db:"id"`
	Label            string    `db:"label"`
	Color            string    `db:"color"`
	LabelDescription string    `db:"label_description"`
	OrganizationId   int       `db:"organization_id"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

type NewCategory struct {
	Label            string
	Color            string
	LabelDescription string
	OrganizationId   int
}

type UpdateCategory struct {
	Id               int
	Label            string
	Color            string
	LabelDescription string
	OrganizationId   int
}
