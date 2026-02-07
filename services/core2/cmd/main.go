package main

import (
	"context"
	"time"

	"planeo/libs/logger"
	"planeo/services/core2/internal/config"
	"planeo/services/core2/internal/domain/category"
	"planeo/services/core2/internal/domain/organization"
	"planeo/services/core2/internal/domain/request"
	"planeo/services/core2/internal/domain/user"
	"planeo/services/core2/internal/infra/keycloak"
	"planeo/services/core2/internal/infra/postgres"
	keycloakClient "planeo/services/core2/pkg/keycloak"
)

// func generateOpenApiSpecs(api huma.API, filename string) {
// 	spec, err := api.OpenAPI().YAML()
// 	if err != nil {
// 		panic(err)
// 	}

// 	if err := os.WriteFile(filename, spec, 0600); err != nil {
// 		panic(err)
// 	}
// }

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

	_ = categoryService
	_ = organizationService
	_ = requestService
	_ = userService
	// // initialize event service
	// eventService, err := events.NewEventService(config.NatsUrl)
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("Failed to connect to NATS")
	// }

	// // initialize application
	// appFactory := setup.NewApplicationFactory()
	// application := appFactory.CreateApplication(config, db, eventService)

	// server := http.Server{
	// 	Addr:              serverConfig,
	// 	Handler:           application.API.Router,
	// 	ReadTimeout:       5 * time.Second,
	// 	WriteTimeout:      5 * time.Second,
	// 	IdleTimeout:       30 * time.Second,
	// 	ReadHeaderTimeout: 2 * time.Second,
	// }

	// generateOpenApiSpecs(application.API.Api, "./docs/open-api-specs.yaml")

	// log.Info().Msgf("Server Running at %s", serverConfig)
	// log.Fatal().Msgf("%v", server.ListenAndServe())
	sleepDuration := 5 * time.Second
	log.Info().Msgf("Sleeping for %s before closing application", sleepDuration)
	time.Sleep(sleepDuration)
	log.Info().Msg("Closing application")
}
