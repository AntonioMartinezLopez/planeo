package keycloak

import (
	"fmt"
	"planeo/libs/request"
)

type CreateUserParams struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
}

func (kc *KeycloakAdminClient) CreateKeycloakUser(groupId string, userData CreateUserParams) error {

	data := map[string]any{
		"firstName":     userData.FirstName,
		"lastName":      userData.LastName,
		"email":         userData.Email,
		"emailVerified": true,
		"enabled":       true,
		"groups":        []string{fmt.Sprintf("/%s", groupId)},
		"credentials":   []map[string]any{{"type": "password", "value": userData.Password, "temporary": false}},
	}

	requestParams := KeycloakRequestParams{
		Method:  request.POST,
		Url:     fmt.Sprintf("%s/admin/realms/%s/users", kc.baseUrl, kc.realm),
		Payload: data,
	}

	return SendRequest(kc, requestParams, nil)

}
