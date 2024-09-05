package organization_management

import (
	"fmt"
	jsonHelper "planeo/api/pkg/json"
	"planeo/api/pkg/request"
)

type OidcAuthenticationResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func AuthenticateAdmin() (string, error) {

	data := map[string]string{
		"grant_type":    "password",
		"username":      "admin",
		"password":      "admin",
		"scope":         "openid profile email",
		"client_id":     "admin-cli",
		"client_secret": "lJPPN8Tn2x7dLkMzpDSCTyMxU8671dwN",
	}

	requestParams := request.HttpRequestParams{
		Method:      request.POST,
		URL:         "http://localhost:8080/realms/master/protocol/openid-connect/token",
		Body:        data,
		ContentType: request.ApplicationFormURLEncoded,
	}

	response, err := request.HttpRequestWithRetry(requestParams)

	if err != nil {
		return "", fmt.Errorf("something went wrong during admin authentication: %s", err.Error())
	}

	if response.StatusCode != 200 {
		fmt.Println(response.Body)
		return "", fmt.Errorf("something went wrong during admin authentication: %d response error from keycloak", response.StatusCode)
	}

	defer response.Body.Close()

	tokens := new(OidcAuthenticationResponse)
	validationError := jsonHelper.DecodeJSONAndValidate(response.Body, tokens, true)

	if validationError != nil {
		fmt.Println("Validation error", validationError)
		return "", validationError
	}

	return tokens.AccessToken, nil
}
