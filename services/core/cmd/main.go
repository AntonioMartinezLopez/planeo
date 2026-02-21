package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"planeo/libs/logger"
	"planeo/services/core/internal/config"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/organization"
	"planeo/services/core/internal/domain/request"
	"planeo/services/core/internal/domain/user"
	coreEvents "planeo/services/core/internal/infra/events"
	"planeo/services/core/internal/infra/keycloak"
	"planeo/services/core/internal/infra/postgres"
	"planeo/services/core/internal/infra/rest"
	keycloakClient "planeo/services/core/pkg/keycloak"

	"github.com/danielgtaylor/huma/v2"
)

func main() {
	// Initialize logger
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	log := logger.New("main")
	ctx := logger.WithContext(context.Background(), log)

	// server configuration
	log.Info().Msg("Loading environment variables")
	config := config.LoadConfig(ctx)
	serverConfig := config.ServerConfig()

	log.Info().Msgf("Server configuration loaded: %s", serverConfig)

	// initialize database
	db := postgres.NewClient(ctx, config.DatabaseConfig())
	defer db.Close()

	// initialize keycloak service
	keycloakClientProps := keycloakClient.KeycloakAdminClientProps{
		BaseUrl:      config.KcBaseUrl,
		Realm:        config.KcIssuer,
		Username:     config.KcAdminUsername,
		Password:     config.KcAdminPassword,
		ClientId:     config.KcAdminClientID,
		ClientSecret: config.KcAdminClientSecret,
	}
	keycloakAdminClient := keycloakClient.NewKeycloakAdminClient(keycloakClientProps)
	keycloakService := keycloak.NewKeycloakService(keycloakAdminClient, config)

	// initialize services
	categoryService := category.NewService(db)
	organizationService := organization.NewService(db)
	requestService := request.NewService(db)
	userService := user.NewService(db, keycloakService)

	// initialize rest server
	srv := rest.New(rest.Config{
		AppName:          "core",
		Version:          "0.0.1",
		ServerAddress:    config.Host,
		OauthIssuerUrl:   config.OauthIssuerUrl(),
		OauthClientID:    config.KcOauthClientID,
		EnableStackTrace: false,
		AllowOrigins:     []string{},
	}, rest.Services{
		UserService:         userService,
		CategoryService:     categoryService,
		OrganizationService: organizationService,
		RequestService:      requestService,
	})

	// initialize event service
	err := coreEvents.InitializeEvents(ctx, config.NatsUrl, coreEvents.Services{RequestService: requestService, CategoryService: categoryService})

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to NATS")
	}

	server := http.Server{
		Addr:              serverConfig,
		Handler:           srv.Router,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	generateOpenApiSpecs(srv.Api, "./docs/open-api-specs.yaml")

	log.Info().Msgf("Server Running at %s", serverConfig)
	log.Fatal().Msgf("%v", server.ListenAndServe())
}

func generateOpenApiSpecs(api huma.API, filename string) {
	spec, err := api.OpenAPI().YAML()
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(filename, spec, 0600); err != nil {
		panic(err)
	}
}
