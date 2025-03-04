package keycloak

import (
	"fmt"
	"planeo/libs/request"
)

func (kc *KeycloakAdminClient) DeleteKeycloakUserClientRoles(ClientUuid string, roleRepresentation []KeycloakClientRole, userId string) error {

	requestParams := KeycloakRequestParams{
		Method:  request.DELETE,
		Url:     fmt.Sprintf("%s/admin/realms/%s/users/%s/role-mappings/clients/%s", kc.baseUrl, kc.realm, userId, ClientUuid),
		Payload: roleRepresentation,
	}

	return SendRequest(kc, requestParams, nil)
}
