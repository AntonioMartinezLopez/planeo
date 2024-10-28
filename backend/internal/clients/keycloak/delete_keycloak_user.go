package keycloak

import (
	"fmt"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) DeleteKeycloakUser(userId string) error {

	requestParams := KeycloakRequestParams{
		Method: request.DELETE,
		Url:    fmt.Sprintf("%s/admin/realms/%s/users/%s", kc.baseUrl, kc.realm, userId),
	}

	return SendRequest(kc, requestParams, nil)
}
