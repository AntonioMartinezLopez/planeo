package keycloak

import (
	"fmt"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) AddKeycloakClientRoleToUser(ClientUuid string, roles []KeycloakClientRole, userId string) error {

	accessToken, err := kc.getAccessToken()

	if err != nil {
		return err
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessToken),
	}

	requestParams := request.HttpRequestParams{
		Method:      request.POST,
		URL:         fmt.Sprintf("%s/admin/realms/%s/users/%s/role-mappings/clients/%s", kc.baseUrl, kc.realm, userId, ClientUuid),
		Headers:     headers,
		ContentType: request.ApplicationJSON,
		Body:        roles,
	}

	response, err := request.HttpRequestWithRetry(requestParams)

	if err != nil {
		return err
	}

	if response.StatusCode != 204 {
		return fmt.Errorf("something went wrong: Fetching Keycloak endpoint resulted in http response %d", response.StatusCode)
	}

	return nil
}
