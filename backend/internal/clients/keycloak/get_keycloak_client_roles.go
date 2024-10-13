package keycloak

import (
	"fmt"
	jsonHelper "planeo/api/pkg/json"
	"planeo/api/pkg/logger"
	"planeo/api/pkg/request"
)

type DefaultRole int

const (
	User DefaultRole = iota
	Planner
	Admin
)

var roles = [...]string{"User", "Planner", "Admin"}

func (me DefaultRole) String() string {
	return roles[me]
}

type KeycloakClientRole struct {
	Id   string `json:"id" validate:"required"`
	Name string `json:"name" validate:"required"`
}

func (kc *KeycloakAdminClient) GetKeycloakClientRoles(clientUuid string) ([]KeycloakClientRole, error) {

	accessToken, err := kc.getAccessToken()

	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	requestParams := request.HttpRequestParams{
		Method:      request.GET,
		URL:         fmt.Sprintf("%s/admin/realms/%s/clients/%s/roles", kc.baseUrl, kc.realm, clientUuid),
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

	var keycloakClientRoles []KeycloakClientRole
	validationError := jsonHelper.DecodeJSONAndValidate(response.Body, &keycloakClientRoles, true)

	if validationError != nil {
		logger.Error("Validation error: %s", validationError)
		return nil, validationError
	}

	return keycloakClientRoles, nil
}
