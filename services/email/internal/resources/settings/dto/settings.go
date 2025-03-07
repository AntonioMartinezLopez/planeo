package dto

import "planeo/services/email/internal/resources/settings/models"

// GET Settings
type GetSettingsInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
}

type GetSettingsOutput struct {
	Body struct {
		Settings []models.Setting `json:"settings" doc:"Array of email settings"`
	}
}

// GET Single Setting
type GetSettingInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	SettingId      int `path:"settingId" doc:"ID of the email setting"`
}

type GetSettingOutput struct {
	Body models.Setting
}

// POST Setting
type CreateSettingInputBody struct {
	Host     string `json:"host" doc:"IMAP host" example:"imap.example.com" validate:"required"`
	Port     string `json:"port" doc:"IMAP port" example:"587" validate:"required"`
	Username string `json:"username" doc:"IMAP username" example:"user@example.com"`
	Password string `json:"password" doc:"IMAP password" example:"password123"`
}

type CreateSettingInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	Body           CreateSettingInputBody
}

// UPDATE Setting
type UpdateSettingInputBody struct {
	Host     string `json:"host" doc:"IMAP host" example:"imap.example.com" validate:"required"`
	Port     string `json:"port" doc:"IMAP port" example:"587" validate:"required"`
	Username string `json:"username" doc:"IMAP username" example:"user@example.com"`
	Password string `json:"password" doc:"IMAP password" example:"password123"`
}

type UpdateSettingInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	SettingId      int `path:"settingId" doc:"ID of the email setting"`
	Body           UpdateSettingInputBody
}

// DELETE Setting
type DeleteSettingInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	SettingId      int `path:"settingId" doc:"ID of the email setting"`
}
