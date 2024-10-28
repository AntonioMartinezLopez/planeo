package keycloak

import (
	"fmt"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakUserById(userId string) (*KeycloakUser, error) {

	requestParams := KeycloakRequestParams{
		Method: request.GET,
		Url:    fmt.Sprintf("%s/admin/realms/%s/users/%s", kc.baseUrl, kc.realm, userId),
	}

	var keycloakUser = new(KeycloakUser)
	err := SendRequest(kc, requestParams, keycloakUser)

	if err != nil {
		return nil, err
	}

	return keycloakUser, nil
}
