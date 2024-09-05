package organization_management

import (
	"fmt"
	jsonHelper "planeo/api/pkg/json"
	"planeo/api/pkg/logger"
	"planeo/api/pkg/request"
)

type KeycloakGroup struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func GetKeycloakGroup(accessToken string, organizationId string) (*KeycloakGroup, error) {

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	requestParams := request.HttpRequestParams{
		Method:      request.GET,
		URL:         fmt.Sprintf("http://localhost:8080/admin/realms/local/group-by-path/%s", organizationId),
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
