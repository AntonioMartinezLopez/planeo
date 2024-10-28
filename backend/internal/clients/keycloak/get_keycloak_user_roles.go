package keycloak

import (
	"fmt"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakUserClientRoles(ClientUuid string, userId string) ([]KeycloakClientRole, error) {

	requestParams := KeycloakRequestParams{
		Method: request.GET,
		Url:    fmt.Sprintf("%s/admin/realms/%s/users/%s/role-mappings/clients/%s", kc.baseUrl, kc.realm, userId, ClientUuid),
	}

	var keycloakClientRoles []KeycloakClientRole
	err := SendRequest(kc, requestParams, &keycloakClientRoles)

	if err != nil {
		return nil, err
	}

	return keycloakClientRoles, nil

}
