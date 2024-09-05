package organization_management

import (
	"fmt"
	jsonHelper "planeo/api/pkg/json"
	"planeo/api/pkg/logger"
	"planeo/api/pkg/request"
)

type KeycloakUser struct {
	Id        string `json:"id" example:"123456" doc:"User identifier within Keycloak"`
	Userame   string `json:"username" example:"user123" doc:"User name"`
	FirstName string `json:"firstName" example:"John" doc:"First name of the user"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

func GetKeycloakUsers(accessToken string, organizationId string) ([]KeycloakUser, error) {

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	// TODO: group information should be cached
	group, err := GetKeycloakGroup(accessToken, organizationId)

	if err != nil {
		return nil, err
	}

	requestParams := request.HttpRequestParams{
		Method:      request.GET,
		URL:         fmt.Sprintf("http://localhost:8080/admin/realms/local/groups/%s/members", group.Id),
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

	return keycloakUsers, nil
}
