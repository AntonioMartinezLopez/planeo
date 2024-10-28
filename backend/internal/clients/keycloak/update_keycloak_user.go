package keycloak

import (
	"fmt"
	"planeo/api/pkg/request"
)

type UpdateUserParams struct {
	Id              string   `json:"id" example:"123456" doc:"User identifier within Keycloak" validate:"required"`
	Userame         string   `json:"username" example:"user123" doc:"User name" binding:"required"`
	FirstName       string   `json:"firstName" example:"John" doc:"First name of the user" validate:"required"`
	LastName        string   `json:"lastName" validate:"required"`
	Email           string   `json:"email" validate:"required"`
	Enabled         bool     `json:"enabled"`
	EmailVerified   bool     `json:"emailVerified"`
	Totp            bool     `json:"totp"`
	RequiredActions []string `json:"requiredActions" validate:"required"`
}

func (kc *KeycloakAdminClient) UpdateKeycloakUser(userId string, userData UpdateUserParams) error {

	requestParams := KeycloakRequestParams{
		Method:  request.PUT,
		Url:     fmt.Sprintf("%s/admin/realms/%s/users/%s", kc.baseUrl, kc.realm, userId),
		Payload: userData,
	}

	return SendRequest(kc, requestParams, nil)
}
