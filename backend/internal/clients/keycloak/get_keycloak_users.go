package keycloak

import (
	"fmt"
	"planeo/api/pkg/request"
)

func (kc *KeycloakAdminClient) GetKeycloakUsers(groupId string) ([]KeycloakUser, error) {

	// TODO: group information should be cached
	group, err := kc.GetKeycloakGroup(groupId)

	if err != nil {
		return nil, err
	}

	var keycloakUsers []KeycloakUser

	requestParams := KeycloakRequestParams{
		Url:     fmt.Sprintf("%s/admin/realms/%s/groups/%s/members", kc.baseUrl, kc.realm, group.Id),
		Method:  request.GET,
		Payload: nil,
	}

	requestError := SendRequest(kc, requestParams, &keycloakUsers)

	if requestError != nil {
		return nil, requestError
	}

	return keycloakUsers, nil
}
