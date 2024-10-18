package keycloak

import (
	"fmt"
	jsonHelper "planeo/api/pkg/json"
	"planeo/api/pkg/logger"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakUserById(userId string) (*KeycloakUser, error) {

	accessToken, err := kc.getAccessToken()

	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	requestParams := request.HttpRequestParams{
		Method:      request.GET,
		URL:         fmt.Sprintf("%s/admin/realms/%s/users/%s", kc.baseUrl, kc.realm, userId),
		Headers:     headers,
		ContentType: request.ApplicationJSON,
	}

	response, err := request.HttpRequestWithRetry(requestParams)

	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("something went wrong: Fetching Keycloak endpoint resulted in http response %d", response.StatusCode)
	}

	defer response.Body.Close()

	var keycloakUser = new(KeycloakUser)
	validationError := jsonHelper.DecodeJSONAndValidate(response.Body, keycloakUser, true)

	if validationError != nil {
		logger.Error("Validation error: %s", validationError)
		return nil, validationError
	}

	return keycloakUser, nil
}
