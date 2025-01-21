package setup

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	cfg "planeo/api/config"
	"planeo/api/internal/middlewares"
	"planeo/api/pkg/db"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Controller interface {
	InitializeRoutes()
}

func registerControllers(config *cfg.ApplicationConfiguration, api huma.API, controllers []Controller) {
	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", config.OauthIssuerUrl())
	api.UseMiddleware(middlewares.AuthMiddleware(api, jwksURL, config.OauthIssuerUrl()))

	type Message struct {
		Alive bool `json:"alive" path:"status" doc:"Status of the API server" `
	}

	type StatusOutput struct {
		Body struct {
			Message
		}
	}

	huma.Register(api, huma.Operation{
		OperationID: "server-status",
		Method:      http.MethodGet,
		Path:        "/status",
		Summary:     "Get the server status",
	}, func(ctx context.Context, input *struct{}) (*StatusOutput, error) {
		resp := &StatusOutput{}
		resp.Body.Message.Alive = true
		return resp, nil
	})

	// initialize new routes
	for _, controller := range controllers {
		controller.InitializeRoutes()
	}

}

func getApiUrl(config *cfg.ApplicationConfiguration) string {
	server := config.ServerConfig()
	containsLocalhost := strings.Contains(server, "localhost")
	if containsLocalhost {
		return strings.Join([]string{"http://", server, "/api"}, "")
	}
	return strings.Join([]string{"https://", server, "/api"}, "")
}

func SetupRouter(config *cfg.ApplicationConfiguration, db *db.DBConnection) *chi.Mux {

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
		controllers := InitializeControllers(&api, config, db)
		registerControllers(config, api, controllers)
	})

	return router
}
