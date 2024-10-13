package main

import (
	"net/http"
	"planeo/api/config"
	"planeo/api/internal/setup"

	"planeo/api/pkg/logger"
	"time"
)

func main() {

	// server configuration
	logger.Log("Loading configuration")
	config.LoadConfig()
	serverConfig := config.ServerConfig()

	// Initialize Router
	router := setup.SetupRouter()

	server := http.Server{
		Addr:              serverConfig,
		Handler:           router,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	logger.Log("Server Running at %s", serverConfig)
	logger.Fatal("%v", server.ListenAndServe())
}
