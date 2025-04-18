package main

import (
	"context"
	"net/http"
	"planeo/libs/db"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/email/config"
	"planeo/services/email/internal/setup"
	"time"

	"github.com/nats-io/nats.go"
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

	// initialize database connection
	db := db.InitializeDatabaseConnection(ctx, config.DatabaseConfig())

	// initialize nats connection
	natsClient, err := events.NewNatsConnector(config.NatsUrl, []nats.Option{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to NATS")
	}

	// initialize application
	appFactory := setup.NewApplicationFactory()
	application := appFactory.CreateApplication(config, db, natsClient)

	server := http.Server{
		Addr:              serverConfig,
		Handler:           application.API.Router,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	log.Info().Msgf("Server Running at %s", serverConfig)
	log.Fatal().Msgf("%v", server.ListenAndServe())
}
