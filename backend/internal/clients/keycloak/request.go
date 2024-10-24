package keycloak

import (
	"fmt"
	"io"
	jsonHelper "planeo/api/pkg/json"
	"planeo/api/pkg/logger"
	"planeo/api/pkg/request"
	"slices"
)

type KeycloakRequestParams struct {
	Url     string
	Method  request.HttpMethod
	Payload any
}

var expectedReturnCodes = map[request.HttpMethod][]int{
	request.POST:   {201, 200, 204},
	request.GET:    {200},
	request.DELETE: {204},
	request.PATCH:  {204},
	request.PUT:    {204},
}

func SendRequest(kc *KeycloakAdminClient, keycloakRequestParams KeycloakRequestParams, data any) error {

	accessToken, err := kc.getAccessToken()

	if err != nil {
		return err
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	requestParams := request.HttpRequestParams{
		Method:      keycloakRequestParams.Method,
		URL:         keycloakRequestParams.Url,
		Headers:     headers,
		ContentType: request.ApplicationJSON,
		Body:        keycloakRequestParams.Payload,
	}

	response, err := request.HttpRequestWithRetry(requestParams)

	if err != nil {
		return err
	}

	defer response.Body.Close()

	expectedStatusCode := slices.Contains(expectedReturnCodes[keycloakRequestParams.Method], response.StatusCode)
	if !expectedStatusCode {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("something went wrong: Fetching Keycloak endpoint resulted in http response %d: %s", response.StatusCode, body)
	}

	if response.ContentLength > 0 && data != nil {
		validationError := jsonHelper.DecodeJSONAndValidate(response.Body, data, true)

		if validationError != nil {
			logger.Error("Validation error: %s", validationError)
			return validationError
		}
	}

	return nil
}
