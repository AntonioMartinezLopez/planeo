package keycloak

import (
	"fmt"
	"planeo/services/core/pkg/request"
)

func (kc *KeycloakAdminClient) AddKeycloakClientRoleToUser(ClientUuid string, roles []KeycloakClientRole, userId string) error {

	requestParams := KeycloakRequestParams{
		Url:     fmt.Sprintf("%s/admin/realms/%s/users/%s/role-mappings/clients/%s", kc.baseUrl, kc.realm, userId, ClientUuid),
		Method:  request.POST,
		Payload: roles,
	}

	return SendRequest(kc, requestParams, nil)
}
