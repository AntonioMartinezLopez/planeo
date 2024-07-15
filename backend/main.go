package main

import (
	"net/http"
	"planeo/api/api/router"
	"planeo/api/config"
	"planeo/api/pkg/logger"
	"time"
)

func main() {

	// Load configuration
	config.LoadConfig()

	// Initialize Router
	router := router.SetupRouter()
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
