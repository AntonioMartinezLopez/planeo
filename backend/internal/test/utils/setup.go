package utils

import (
	"context"
	"fmt"
	"planeo/api/config"
	"testing"

	keycloak "github.com/stillya/testcontainers-keycloak"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type IntegrationTestEnvironment struct {
	KeycloakContainer *keycloak.KeycloakContainer
	PostgresContainer *postgres.PostgresContainer
	Configuration     *config.ApplicationConfiguration
}

func NewIntegrationTestEnvironment(t *testing.T) *IntegrationTestEnvironment {

	// Start containers
	var keycloakContainer *keycloak.KeycloakContainer
	var postgresContainer *postgres.PostgresContainer

	keycloakContainer, err := NewKeycloakContainer(context.Background())
	if err != nil {
		panic(err)
	}

	postgresContainer, err = StartPostgresContainer(context.Background())
	if err != nil {
		panic(err)
	}

	// load dynamic ports
	postresPort, err := postgresContainer.MappedPort(context.Background(), "5432")
	if err != nil {
		t.Error(err)
	}
	keycloakPort, err := keycloakContainer.MappedPort(context.Background(), "8080")
	if err != nil {
		t.Error(err)
	}

	config := config.LoadConfig("../../.env.test.template")
	config.DbPort = postresPort.Port()
	config.KcBaseUrl = fmt.Sprintf("http://localhost:%s", keycloakPort.Port())

	env := &IntegrationTestEnvironment{
		KeycloakContainer: keycloakContainer,
		PostgresContainer: postgresContainer,
		Configuration:     config,
	}

	t.Cleanup(func() {
		ctx := context.Background()
		err := env.KeycloakContainer.Terminate(ctx)
		if err != nil {
			panic(err)
		}

		err = env.PostgresContainer.Terminate(ctx)
		if err != nil {
			panic(err)
		}
	})

	return env
}

func (env *IntegrationTestEnvironment) GetUserSession(username string, password string) (*UserSession, error) {
	return GetUserSession(env.KeycloakContainer, username, password)
}
