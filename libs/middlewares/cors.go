package middlewares

import (
	"net/http"

	"github.com/go-chi/cors"
)

func Cors(origins []string) func(http.Handler) http.Handler {
	allowedOrigins := []string{
		"0.0.0.0",
		"localhost",
		"*",
		"localhost:8080",
	}

	if len(origins) > 0 {
		allowedOrigins = origins
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Access-Control-Allow-Origin", "Access-Control-Allow-Method", "Access-Control-Allow-Headers"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
}
