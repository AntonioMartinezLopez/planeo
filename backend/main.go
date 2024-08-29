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
	serverConfig := config.ServerConfig()
	logger.Log("Server Running at %s", serverConfig)

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

	logger.Fatal("%v", server.ListenAndServe())
}
