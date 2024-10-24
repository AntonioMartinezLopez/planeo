package keycloak

import (
	"fmt"
	"io"
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

	accessToken, err := kc.getAccessToken()

	if err != nil {
		return err
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	requestParams := request.HttpRequestParams{
		Method:      request.PUT,
		URL:         fmt.Sprintf("%s/admin/realms/%s/users/%s", kc.baseUrl, kc.realm, userId),
		Headers:     headers,
		ContentType: request.ApplicationJSON,
		Body:        userData,
	}

	response, err := request.HttpRequestWithRetry(requestParams)

	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != 204 {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("something went wrong: Fetching Keycloak endpoint resulted in http response %d: %s", response.StatusCode, body)
	}

	return nil
}
