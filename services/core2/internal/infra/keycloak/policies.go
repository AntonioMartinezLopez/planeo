package keycloak

import "planeo/services/core2/pkg/keycloak"

type KeycloakAdminClientInterface interface {
	GetKeycloakUserGroups(userId string) ([]keycloak.KeycloakGroup, error)
}

func UserInOrganization(kc KeycloakAdminClientInterface, organizationKeycloakId string, userKeycloakId string) bool {

	groups, err := kc.GetKeycloakUserGroups(userKeycloakId)

	if err != nil {
		return false
	}

	for _, group := range groups {
		if group.Path == "/"+organizationKeycloakId {
			return true
		}
	}

	return false
}
