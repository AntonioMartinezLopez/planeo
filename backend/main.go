package main

import (
	"net/http"
	"planeo/api/config"
	"planeo/api/internal/setup"

	"planeo/api/pkg/logger"
	"time"
)

func main() {

	// Load configuration
	config.LoadConfig()

	// Initialize Router
	router := setup.SetupRouter()
	server := http.Server{
		Addr:              config.ServerConfig(),
		Handler:           router,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	logger.Fatal("%v", server.ListenAndServe())
}
