package main

import (
	"context"
	"net/http"
	"planeo/libs/db"
	"planeo/libs/logger"
	"planeo/services/email/config"
	"planeo/services/email/internal/setup"
	"time"
)

func main() {

	ctx := context.Background()

	// Initialize logger
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	logger := logger.New("main")

	// server configuration
	logger.Info().Msg("Loading environment variables")
	config := config.LoadConfig()
	serverConfig := config.ServerConfig()

	// initialize database connection
	db := db.InitializeDatabaseConnection(ctx, config.DatabaseConfig())

	// initilalize cron service

	// initialize api
	router := setup.SetupRouter(config, db)

	server := http.Server{
		Addr:              serverConfig,
		Handler:           router,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	logger.Info().Msgf("Server Running at %s", serverConfig)
	logger.Fatal().Msgf("%v", server.ListenAndServe())
}
