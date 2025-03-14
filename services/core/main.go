package main

import (
	"context"
	"net/http"
	"planeo/services/core/config"
	"planeo/services/core/internal/setup"

	"planeo/libs/db"
	"planeo/libs/logger"
	"time"
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

	log.Info().Msgf("Server Running at %s", serverConfig)
	log.Fatal().Msgf("%v", server.ListenAndServe())
}
