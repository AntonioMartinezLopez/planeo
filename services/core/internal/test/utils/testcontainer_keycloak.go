package utils

import (
	"context"
	"fmt"
	"path/filepath"
	jsonHelper "planeo/libs/json"
	"planeo/libs/request"
	"time"

	keycloak "github.com/stillya/testcontainers-keycloak"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type UserSession struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

func NewKeycloakContainer(ctx context.Context) (*keycloak.KeycloakContainer, error) {
	realmPath, _ := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "auth", "local", "realm.json"))
	return keycloak.Run(ctx,
		"quay.io/keycloak/keycloak:25.0.2",
		testcontainers.WithHostPortAccess(8080),
		testcontainers.WithWaitStrategyAndDeadline(5*time.Minute, wait.ForAll(
			wait.ForHTTP("/realms/local/protocol/openid-connect/certs").WithPort("8080"),
		)),
		keycloak.WithContextPath("/"),
		keycloak.WithRealmImportFile(realmPath),
		keycloak.WithAdminUsername("admin"),
		keycloak.WithAdminPassword("password"),
	)
}

func GetUserSession(k *keycloak.KeycloakContainer, username string, password string) (*UserSession, error) {

	if !k.IsRunning() {
		return nil, nil
	}

	url, err := k.GetAuthServerURL(context.Background())

	if err != nil {
		return nil, fmt.Errorf("something went wrong during admin authentication: %s", err.Error())
	}

	data := map[string]string{
		"grant_type":    "password",
		"username":      username,
		"password":      password,
		"scope":         "openid profile email",
		"client_id":     "local",
		"client_secret": "t4VlYX9CJIN3VTrlb5nRMXT8Qjr9SBdu",
	}

	requestParams := request.HttpRequestParams{
		Method:      request.POST,
		URL:         fmt.Sprintf("%s/realms/local/protocol/openid-connect/token", url),
		Body:        data,
		ContentType: request.ApplicationFormURLEncoded,
	}

	response, err := request.HttpRequestWithRetry(requestParams)

	if err != nil {
		return nil, fmt.Errorf("something went wrong during admin authentication: %s", err.Error())
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("something went wrong during admin authentication: %d response error from keycloak", response.StatusCode)
	}

	defer response.Body.Close()

	session := new(UserSession)
	validationError := jsonHelper.DecodeJSONAndValidate(response.Body, session, true)

	if validationError != nil {
		fmt.Println("Validation error", validationError)
		return nil, validationError
	}

	return session, nil

}
