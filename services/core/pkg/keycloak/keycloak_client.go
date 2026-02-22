package keycloak

import (
	"time"
)

type KeycloakAdminClientProps struct {
	BaseUrl      string
	Realm        string
	Username     string
	Password     string
	ClientId     string
	ClientSecret string
}

type KeycloakAdminClient struct {
	baseUrl      string
	realm        string
	username     string
	password     string
	clientId     string
	clientSecret string
	session      *AdminKeycloakSession
}

func NewKeycloakAdminClient(props KeycloakAdminClientProps) *KeycloakAdminClient {

	return &KeycloakAdminClient{
		baseUrl:      props.BaseUrl,
		realm:        props.Realm,
		username:     props.Username,
		password:     props.Password,
		clientId:     props.ClientId,
		clientSecret: props.ClientSecret,
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
