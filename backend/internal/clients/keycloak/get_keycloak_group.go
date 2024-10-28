package keycloak

import (
	"fmt"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakGroup(organizationId string) (*KeycloakGroup, error) {

	requestParams := KeycloakRequestParams{
		Method: request.GET,
		Url:    fmt.Sprintf("%s/admin/realms/%s/group-by-path/%s", kc.baseUrl, kc.realm, organizationId),
	}

	keycloakGroup := new(KeycloakGroup)
	err := SendRequest(kc, requestParams, keycloakGroup)

	if err != nil {
		return nil, err
	}

	return keycloakGroup, nil
}
