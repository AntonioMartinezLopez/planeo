package settings

import "planeo/services/email/internal/domain/setting"

// GET Settings
type GetSettingsInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
}

type GetSettingsOutput struct {
	Body struct {
		Settings []setting.Setting `json:"settings" doc:"Array of email settings"`
	}
}

// POST Setting
type CreateSettingInputBody struct {
	Host     string `json:"host" doc:"IMAP host" example:"imap.example.com" validate:"required"`
	Port     int    `json:"port" doc:"IMAP port" example:"993" validate:"required"`
	Username string `json:"username" doc:"IMAP username" example:"user@example.com"`
	Password string `json:"password" doc:"IMAP password" example:"password123"`
}

type CreateSettingInput struct {
	OrganizationId int `path:"organizationId" doc:"ID of the organization"`
	Body           CreateSettingInputBody
}

// PUT Setting
type UpdateSettingInputBody struct {
	Host     string `json:"host" doc:"IMAP host" example:"imap.example.com" validate:"required"`
	Port     int    `json:"port" doc:"IMAP port" example:"993" validate:"required"`
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

// Test Connection
type TestSettingInput struct {
	Body CreateSettingInputBody
}
