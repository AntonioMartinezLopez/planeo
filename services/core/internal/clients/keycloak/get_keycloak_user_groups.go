package keycloak

import (
	"fmt"
	"planeo/services/core/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakUserGroups(userId string) ([]KeycloakGroup, error) {

	requestParams := KeycloakRequestParams{
		Method: request.GET,
		Url:    fmt.Sprintf("%s/admin/realms/%s/users/%s/groups", kc.baseUrl, kc.realm, userId),
	}

	var keycloakGroups []KeycloakGroup
	err := SendRequest(kc, requestParams, &keycloakGroups)

	if err != nil {
		return nil, err
	}

	return keycloakGroups, nil

}
