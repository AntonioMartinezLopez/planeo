package policies

import (
	"planeo/api/internal/clients/keycloak"
	"planeo/api/pkg/logger"
)

type KeycloakAdminClientInterface interface {
	GetKeycloakUserGroups(userId string) ([]keycloak.KeycloakGroup, error)
}

func UserInOrganisation(kc KeycloakAdminClientInterface, organizationId string, userKeycloakId string) bool {

	groups, err := kc.GetKeycloakUserGroups(userKeycloakId)

	if err != nil {
		logger.Error("Error getting user groups %s", err.Error())
	}

	for _, group := range groups {
		if group.Path == "/"+organizationId {
			return true
		}
	}

	return false
}
