package keycloak

import (
	"fmt"
	"planeo/services/core/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakGroup(groupId string) (*KeycloakGroup, error) {

	requestParams := KeycloakRequestParams{
		Method: request.GET,
		Url:    fmt.Sprintf("%s/admin/realms/%s/group-by-path/%s", kc.baseUrl, kc.realm, groupId),
	}

	keycloakGroup := new(KeycloakGroup)
	err := SendRequest(kc, requestParams, keycloakGroup)

	if err != nil {
		return nil, err
	}

	return keycloakGroup, nil
}
