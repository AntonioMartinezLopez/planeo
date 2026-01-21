package keycloak

import (
	"fmt"
	jsonHelper "planeo/libs/json"
	"planeo/libs/request"
)

func (kc *KeycloakAdminClient) AuthenticateAdmin() error {

	data := map[string]string{
		"grant_type":    "password",
		"username":      kc.username,
		"password":      kc.password,
		"scope":         "openid profile email",
		"client_id":     kc.clientId,
		"client_secret": kc.clientSecret,
	}

	requestParams := request.HttpRequestParams{
		Method:      request.POST,
		URL:         fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", kc.baseUrl),
		Body:        data,
		ContentType: request.ApplicationFormURLEncoded,
	}

	response, err := request.HttpRequestWithRetry(requestParams)

	if err != nil {
		return fmt.Errorf("something went wrong during admin authentication: %s", err.Error())
	}

	if response.StatusCode != 200 {
		return fmt.Errorf("something went wrong during admin authentication: %d response error from keycloak", response.StatusCode)
	}

	defer response.Body.Close()

	session := new(AdminKeycloakSession)
	validationError := jsonHelper.DecodeJSONAndValidate(response.Body, session, true)

	if validationError != nil {
		fmt.Println("Validation error", validationError)
		return validationError
	}

	kc.session = session

	return nil
}
