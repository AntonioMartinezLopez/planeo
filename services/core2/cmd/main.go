package main

import (
	"context"

	"planeo/libs/db"
	"planeo/libs/logger"
	"planeo/services/core2/config"
	"planeo/services/core2/internal/infra/postgres"
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

	// initialize database connection
	db := db.InitializeDatabaseConnection(ctx, config.DatabaseConfig())

	// initialize
	postgresClient := postgres.NewClient(db)
	defer postgresClient.Close()

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
}
