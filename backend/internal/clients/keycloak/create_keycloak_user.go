package keycloak

import (
	"fmt"
	"io"
	"planeo/api/pkg/request"
)

type CreateUserParams struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
}

func (kc *KeycloakAdminClient) CreateKeycloakUser(organizationId string, userData CreateUserParams) error {

	accessToken, err := kc.getAccessToken()

	if err != nil {
		return err
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	data := map[string]any{
		"firstName":     userData.FirstName,
		"lastName":      userData.LastName,
		"email":         userData.Email,
		"emailVerified": true,
		"enabled":       true,
		"groups":        []string{fmt.Sprintf("/%s", organizationId)},
		"credentials":   []map[string]any{{"type": "password", "value": userData.Password, "temporary": false}},
	}

	requestParams := request.HttpRequestParams{
		Method:      request.POST,
		URL:         fmt.Sprintf("%s/admin/realms/%s/users", kc.baseUrl, kc.realm),
		Headers:     headers,
		ContentType: request.ApplicationJSON,
		Body:        data,
	}

	response, err := request.HttpRequestWithRetry(requestParams)

	if err != nil {
		return err
	}

	if response.StatusCode != 201 {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("something went wrong: Fetching Keycloak endpoint resulted in http response %d: %s", response.StatusCode, body)
	}

	defer response.Body.Close()
	return nil
}
