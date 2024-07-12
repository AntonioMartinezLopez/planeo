package main

import (
	"fmt"
	"net/http"
	"os"
	"planeo/api/config"
	"planeo/api/pkg/logger"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {

	// Load configuration
	config.LoadConfig()

	// Start server
	port := os.Getenv("PORT")
	logger.Log("Starting server on port: %s", port)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})
	http.ListenAndServe(fmt.Sprintf(":%s", port), r)
}
