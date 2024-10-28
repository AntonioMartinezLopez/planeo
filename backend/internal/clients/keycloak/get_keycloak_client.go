package keycloak

import (
	"fmt"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakClient(clientId string) (*KeycloakClient, error) {

	queries := map[string]string{
		"clientId": clientId,
	}

	requestParams := KeycloakRequestParams{
		Method:      request.GET,
		Url:         fmt.Sprintf("%s/admin/realms/%s/clients", kc.baseUrl, kc.realm),
		QueryParams: queries,
	}

	var keycloakClients []KeycloakClient = make([]KeycloakClient, 0)
	err := SendRequest(kc, requestParams, &keycloakClients)

	if err != nil {
		return nil, err
	}

	if len(keycloakClients) < 1 {
		return nil, fmt.Errorf("client with clientId %s not found", clientId)
	}

	keycloakClient := keycloakClients[0]
	return &keycloakClient, nil
}
