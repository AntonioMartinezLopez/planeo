package keycloak

import (
	"fmt"
	jsonHelper "planeo/api/pkg/json"
	"planeo/api/pkg/logger"
	"planeo/api/pkg/request"
)

type KeycloakClient struct {
	Id       string `json:"id" validate:"required"`
	ClientId string `json:"clientId" validate:"reqzured"`
}

func (kc *KeycloakAdminClient) GetKeycloakClient(organizationId string) (*KeycloakClient, error) {

	accessToken, err := kc.getAccessToken()

	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	queries := map[string]string{
		"clientId": organizationId,
	}

	requestParams := request.HttpRequestParams{
		Method:      request.GET,
		URL:         fmt.Sprintf("%s/admin/realms/%s/clients", kc.baseUrl, kc.realm),
		Headers:     headers,
		QueryParams: queries,
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

	keycloakClients := make([]KeycloakClient, 0)
	validationError := jsonHelper.DecodeJSONAndValidate(response.Body, &keycloakClients, true)

	if validationError != nil {
		logger.Error("Validation error: %s", validationError)
		return nil, validationError
	}

	if len(keycloakClients) < 1 {
		return nil, fmt.Errorf("client with clientId %s not found", organizationId)
	}

	keycloakClient := keycloakClients[0]
	return &keycloakClient, nil
}
