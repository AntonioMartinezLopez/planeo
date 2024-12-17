package models

type Role struct {
	Id   string `json:"id" doc:"ID of the role to be assigned to the user"`
	Name string `json:"name" doc:"Name of the role to be assigned to the user" example:"User"`
}
