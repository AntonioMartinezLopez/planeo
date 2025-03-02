package keycloak

import (
	"planeo/services/core/config"
	"time"
)

type KeycloakAdminClient struct {
	baseUrl      string
	realm        string
	username     string
	password     string
	clientId     string
	clientSecret string
	session      *AdminKeycloakSession
}

func NewKeycloakAdminClient(configuration config.ApplicationConfiguration) *KeycloakAdminClient {
	return &KeycloakAdminClient{
		baseUrl:      configuration.KcBaseUrl,
		realm:        configuration.KcIssuer,
		username:     configuration.KcAdminUsername,
		password:     configuration.KcAdminPassword,
		clientId:     configuration.KcAdminClientID,
		clientSecret: configuration.KcAdminClientSecret,
	}
}

func (kc *KeycloakAdminClient) getAccessToken() (string, error) {
	currentTime := time.Now().Unix()
	if kc.session == nil || currentTime > kc.session.ExpiresIn {
		error := kc.AuthenticateAdmin()

		if error != nil {
			return "", error
		}
	}
	return kc.session.AccessToken, nil
}
