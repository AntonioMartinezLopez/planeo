package server

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

type Config struct {
	AppName          string
	Version          string
	ServerAddress    string
	OauthIssuerUrl   string
	EnableStackTrace bool
	AllowOrigins     []string
}

type Server struct {
	api    huma.API
	router *chi.Mux
	config Config
}

func New(config Config) *Server {
	router := chi.NewRouter()
	router.Use(middlewares.LoggerMiddleware)
	router.Use(middleware.Recoverer)
	router.Use(middlewares.Cors(config.AllowOrigins))

	humaConfig := huma.DefaultConfig(config.AppName, config.Version)
	humaConfig.Components.SecuritySchemes = getSecuritySchemes(config)
	humaConfig.Servers = []*huma.Server{
		{URL: getApiUrl(config)},
	}

	apiRouter := chi.NewRouter()
	router.Mount("/api/v1", apiRouter)
	api := humachi.New(apiRouter, humaConfig)

	return &Server{
		api:    api,
		router: router,
		config: config,
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

func getApiUrl(config Config) string {
	server := config.ServerAddress
	containsLocalhost := strings.Contains(server, "localhost")
	if containsLocalhost {
		return strings.Join([]string{"http://", server, "/api"}, "")
	}
	return strings.Join([]string{"https://", server, "/api"}, "")
}

func getSecuritySchemes(config Config) map[string]*huma.SecurityScheme {
	return map[string]*huma.SecurityScheme{
		"bearer": {
			Type: "oauth2",
			Flows: &huma.OAuthFlows{
				AuthorizationCode: &huma.OAuthFlow{
					AuthorizationURL: fmt.Sprintf("%s/protocol/openid-connect/authorize", config.OauthIssuerUrl),
					TokenURL:         fmt.Sprintf("%s/protocol/openid-connect/token", config.OauthIssuerUrl),
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
