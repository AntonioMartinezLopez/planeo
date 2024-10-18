package keycloak

import (
	"errors"
	"fmt"
	jsonHelper "planeo/api/pkg/json"
	"planeo/api/pkg/logger"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakUserByEmail(email string) (*KeycloakUser, error) {

	accessToken, err := kc.getAccessToken()

	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	requestParams := request.HttpRequestParams{
		Method:      request.GET,
		URL:         fmt.Sprintf("%s/admin/realms/%s/users/?email=%s&max=1", kc.baseUrl, kc.realm, email),
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

	var keycloakUsers []KeycloakUser
	validationError := jsonHelper.DecodeJSONAndValidate(response.Body, &keycloakUsers, true)

	if validationError != nil {
		logger.Error("Validation error: %s", validationError)
		return nil, validationError
	}

	if len(keycloakUsers) != 1 {
		return nil, errors.New("user not found")
	}

	return &keycloakUsers[0], nil
}
