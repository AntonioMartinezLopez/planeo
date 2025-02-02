package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// create environment
	env := &IntegrationTestEnvironment{
		KeycloakContainer: keycloakContainer,
		PostgresContainer: postgresContainer,
		Configuration:     config,
	}

	// run migrations
	err = env.MigrateDatabase(false)

	if err != nil {
		t.Error(err.Error())
		panic(err)
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

func (env *IntegrationTestEnvironment) MigrateDatabase(tearDown bool) error {

	operation := "up"

	if tearDown {
		operation = "down"
	}

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	fmt.Println(exPath)

	absPath, _ := filepath.Abs(filepath.Join("..", "..", "..", "db", "migrations"))
	migrationsDir := filepath.Join("..", "..", "..", "db", "migrations")
	println(absPath, migrationsDir)
	cmd := exec.Command("goose", "-dir", migrationsDir, "postgres", fmt.Sprintf("postgres://planeo:planeo@127.0.0.1:%s/planeo?sslmode=disable",
		env.Configuration.DbPort), operation)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	if err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}

	return nil
}

func (env *IntegrationTestEnvironment) ResetDatabase() error {
	err := env.MigrateDatabase(true)
	if err != nil {
		return err
	}
	return env.MigrateDatabase(false)
}
