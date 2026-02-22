package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"planeo/services/core/internal/config"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/organization"
	"planeo/services/core/internal/domain/request"
	"planeo/services/core/internal/domain/user"
	"planeo/services/core/internal/infra/keycloak"
	"planeo/services/core/internal/infra/postgres"
	"planeo/services/core/internal/infra/rest"
	keycloakClient "planeo/services/core/pkg/keycloak"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	keycloakContainer "github.com/stillya/testcontainers-keycloak"
	postgresContainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type IntegrationTestEnvironment struct {
	KeycloakContainer *keycloakContainer.KeycloakContainer
	PostgresContainer *postgresContainer.PostgresContainer
	Configuration     *config.ApplicationConfiguration
	Server            *rest.Server
	DB                *postgres.Client
	Api               humatest.TestAPI
}

func NewIntegrationTestEnvironment(t *testing.T) *IntegrationTestEnvironment {

	// Start containers
	var keycloakCont *keycloakContainer.KeycloakContainer
	var postgresCont *postgresContainer.PostgresContainer

	keycloakCont, err := NewKeycloakContainer(context.Background())
	if err != nil {
		fmt.Println("failed to start keycloak container")
		panic(err)
	}

	postgresCont, err = StartPostgresContainer(context.Background())
	if err != nil {
		fmt.Println("failed to start postgres container")
		panic(err)
	}

	// load dynamic ports
	postgresPort, err := postgresCont.MappedPort(context.Background(), "5432")
	if err != nil {
		t.Error(err)
	}
	keycloakPort, err := keycloakCont.MappedPort(context.Background(), "8080")
	if err != nil {
		t.Error(err)
	}

	// Load configuration
	cfg := config.LoadConfig(context.Background(), "../../../.env.test.template")
	cfg.DbPort = postgresPort.Port()
	cfg.KcBaseUrl = fmt.Sprintf("http://localhost:%s", keycloakPort.Port())

	// create environment
	env := &IntegrationTestEnvironment{
		KeycloakContainer: keycloakCont,
		PostgresContainer: postgresCont,
		Configuration:     cfg,
	}

	// run migrations
	err = env.MigrateDatabase(false)
	if err != nil {
		t.Error(err.Error())
		panic(err)
	}

	// Initialize database
	db := postgres.NewClient(context.Background(), cfg.DatabaseConfig())
	env.DB = db

	// Initialize keycloak service
	keycloakClientProps := keycloakClient.KeycloakAdminClientProps{
		BaseUrl:      cfg.KcBaseUrl,
		Realm:        cfg.KcIssuer,
		Username:     cfg.KcAdminUsername,
		Password:     cfg.KcAdminPassword,
		ClientId:     cfg.KcAdminClientID,
		ClientSecret: cfg.KcAdminClientSecret,
	}
	keycloakAdminClient := keycloakClient.NewKeycloakAdminClient(keycloakClientProps)
	keycloakService := keycloak.NewKeycloakService(keycloakAdminClient, cfg)

	// Initialize all services
	categoryService := category.NewService(db)
	organizationService := organization.NewService(db)
	requestService := request.NewService(db)
	userService := user.NewService(db, keycloakService)

	// Initialize REST server using humatest
	_, testApi := humatest.New(t)

	// Initialize routes using the public InitRoutes function
	restConfig := rest.Config{
		AppName:          "core",
		Version:          "0.0.1",
		ServerAddress:    cfg.Host,
		OauthIssuerUrl:   cfg.OauthIssuerUrl(),
		OauthClientID:    cfg.KcOauthClientID,
		EnableStackTrace: false,
		AllowOrigins:     []string{},
	}

	restServices := rest.Services{
		UserService:         userService,
		CategoryService:     categoryService,
		OrganizationService: organizationService,
		RequestService:      requestService,
	}

	// Use the public InitRoutes function
	rest.InitRoutes(testApi, restConfig, restServices)

	env.Api = testApi

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

		env.DB.Close()
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

	migrationsDir, _ := filepath.Abs(filepath.Join("..", "..", "..", "internal", "infra", "postgres", "migrations"))
	cmd := exec.Command("goose", "-dir", migrationsDir, "postgres", fmt.Sprintf("postgres://planeo:planeo@127.0.0.1:%s/planeo?sslmode=disable",
		env.Configuration.DbPort), operation)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

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
