package keycloak

import (
	"errors"
	"fmt"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakUserByEmail(email string) (*KeycloakUser, error) {

	requestParams := KeycloakRequestParams{
		Method: request.GET,
		Url:    fmt.Sprintf("%s/admin/realms/%s/users/?email=%s&max=1", kc.baseUrl, kc.realm, email),
	}

	var keycloakUsers = make([]KeycloakUser, 0)
	err := SendRequest(kc, requestParams, &keycloakUsers)

	if err != nil {
		return nil, err
	}

	if len(keycloakUsers) != 1 {
		return nil, errors.New("user not found")
	}

	return &keycloakUsers[0], nil
}
