package main

import (
	"context"
	"fmt"
	"net/http"
	"planeo/api/config"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/setup"
	"strings"

	"planeo/api/pkg/db"
	"planeo/api/pkg/logger"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func getApiUrl(config *config.ApplicationConfiguration) string {
	server := config.ServerConfig()
	containsLocalhost := strings.Contains(server, "localhost")
	if containsLocalhost {
		return strings.Join([]string{"http://", server, "/api"}, "")
	}
	return strings.Join([]string{"https://", server, "/api"}, "")
}

func main() {

	ctx := context.Background()

	// server configuration
	logger.Log("Loading configuration")
	config := config.LoadConfig("./.env")
	serverConfig := config.ServerConfig()

	// initialize database connection
	db := db.InitializeDatabaseConnection(ctx, config.DatabaseConfig())

	// Initialize Router
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middlewares.Cors())

	router.Route("/api", func(r chi.Router) {

		humaConfig := huma.DefaultConfig("Planeo API", "0.0.1")
		humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
			// // Example Authorization Code flow.
			"bearer": {
				Type: "oauth2",
				Flows: &huma.OAuthFlows{
					AuthorizationCode: &huma.OAuthFlow{
						AuthorizationURL: fmt.Sprintf("%s/protocol/openid-connect/authorize", config.OauthIssuerUrl()),
						TokenURL:         fmt.Sprintf("%s/protocol/openid-connect/token", config.OauthIssuerUrl()),
						Scopes: map[string]string{
							"openid":  "Scope for requesting OpenID token",
							"profile": "Scope for including user profile",
							"email":   "Scope for providing email information",
						},
					},
				},
			},
		}

		humaConfig.Servers = []*huma.Server{
			{URL: getApiUrl(config)},
		}
		api := humachi.New(r, humaConfig)
		setup.SetupRoutes(api, config, db)
	})

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
