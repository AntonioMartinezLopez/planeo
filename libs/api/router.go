package api

import (
	"context"
	"fmt"
	"net/http"
	"planeo/libs/middlewares"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type RouterConfig interface {
	ServerConfig() string
	OauthIssuerUrl() string
}

type Controller interface {
	InitializeRoutes()
}

// SetupRouter creates and configures a chi Router with common middleware
func SetupRouter(config RouterConfig, basePath string, setupFunc func(r chi.Router, api huma.API)) *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middlewares.Cors())

	router.Route(basePath, func(r chi.Router) {
		humaConfig := huma.DefaultConfig("Planeo API", "0.0.1")
		humaConfig.Components.SecuritySchemes = getSecuritySchemes(config)
		humaConfig.Servers = []*huma.Server{
			{URL: getApiUrl(config)},
		}

		api := humachi.New(r, humaConfig)
		setupFunc(r, api)
	})

	return router
}

// RegisterControllers registers common controllers and endpoints
func RegisterControllers(config RouterConfig, api huma.API, controllers []Controller, middlewareSetupFunc func(api huma.API)) {
	// Register common status endpoint
	registerStatusEndpoint(api)

	// Initialize middleware
	if middlewareSetupFunc != nil {
		middlewareSetupFunc(api)
	}

	// Initialize controllers
	for _, controller := range controllers {
		controller.InitializeRoutes()
	}
}

func registerStatusEndpoint(api huma.API) {
	type Message struct {
		Alive bool `json:"alive" path:"status" doc:"Status of the API server"`
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
}

func getApiUrl(config RouterConfig) string {
	server := config.ServerConfig()
	containsLocalhost := strings.Contains(server, "localhost")
	if containsLocalhost {
		return strings.Join([]string{"http://", server, "/api"}, "")
	}
	return strings.Join([]string{"https://", server, "/api"}, "")
}

func getSecuritySchemes(config RouterConfig) map[string]*huma.SecurityScheme {
	return map[string]*huma.SecurityScheme{
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
}
