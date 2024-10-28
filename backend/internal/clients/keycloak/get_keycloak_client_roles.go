package keycloak

import (
	"fmt"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakClientRoles(clientUuid string) ([]KeycloakClientRole, error) {

	requestParams := KeycloakRequestParams{
		Method: request.GET,
		Url:    fmt.Sprintf("%s/admin/realms/%s/clients/%s/roles", kc.baseUrl, kc.realm, clientUuid),
	}

	var keycloakClientRoles []KeycloakClientRole

	requestError := SendRequest(kc, requestParams, &keycloakClientRoles)

	if requestError != nil {
		return nil, requestError
	}

	return keycloakClientRoles, nil
}
