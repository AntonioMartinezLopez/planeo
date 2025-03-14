package models

import "time"

type Request struct {
	Id             int       `db:"id"`
	Text           string    `db:"text"`
	Name           string    `db:"name"`
	Email          string    `db:"email"`
	Address        string    `db:"address"`
	Telephone      string    `db:"telephone"`
	Closed         bool      `db:"closed"`
	CategoryId     *int      `db:"category_id"`
	OrganizationId int       `db:"organization_id"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type NewRequest struct {
	Text           string
	Name           string
	Email          string
	Address        string
	Telephone      string
	Closed         bool
	CategoryId     int
	OrganizationId int
}

type UpdateRequest struct {
	Id             int
	Text           string
	Name           string
	Email          string
	Address        string
	Telephone      string
	Closed         bool
	CategoryId     int
	OrganizationId int
}
