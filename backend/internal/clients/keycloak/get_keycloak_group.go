package keycloak

import (
	"fmt"
	jsonHelper "planeo/api/pkg/json"
	"planeo/api/pkg/logger"
	"planeo/api/pkg/request"
)

type KeycloakGroup struct {
	Id   string `json:"id" validate:"required"`
	Name string `json:"name" validate:"required"`
	Path string `json:"path" validate:"required"`
}

func (kc *KeycloakAdminClient) GetKeycloakGroup(organizationId string) (*KeycloakGroup, error) {

	accessToken, err := kc.getAccessToken()

	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	requestParams := request.HttpRequestParams{
		Method:      request.GET,
		URL:         fmt.Sprintf("%s/admin/realms/%s/group-by-path/%s", kc.baseUrl, kc.realm, organizationId),
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

	keycloakGroup := new(KeycloakGroup)
	validationError := jsonHelper.DecodeJSONAndValidate(response.Body, keycloakGroup, true)

	if validationError != nil {
		logger.Error("Validation error: %s", validationError)
		return nil, validationError
	}

	return keycloakGroup, nil
}
