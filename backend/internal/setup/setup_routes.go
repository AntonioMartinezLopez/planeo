package setup

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	cfg "planeo/api/config"
	"planeo/api/internal/announcement"
	"planeo/api/internal/group"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/organization_management"
	"planeo/api/internal/task"
	"planeo/api/internal/user"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Controller interface {
	InitializeRoutes()
}

func registerControllers(api huma.API, controllers []Controller) {
	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", cfg.Config.OAuthIssuer)
	api.UseMiddleware(middlewares.AuthMiddleware(api, jwksURL))

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

func getApiUrl() string {
	config := cfg.ServerConfig()
	containsLocalhost := strings.Contains(config, "localhost")
	if containsLocalhost {
		return strings.Join([]string{"http://", config, "/api"}, "")
	}
	return strings.Join([]string{"https://", config, "/api"}, "")
}

func SetupRouter() *chi.Mux {

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middlewares.Cors())

	router.Route("/api", func(r chi.Router) {

		config := huma.DefaultConfig("Planeo API", "0.0.1")
		config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
			// // Example Authorization Code flow.
			"bearer": {
				Type: "oauth2",
				Flows: &huma.OAuthFlows{
					AuthorizationCode: &huma.OAuthFlow{
						AuthorizationURL: fmt.Sprintf("%s/protocol/openid-connect/authorize", cfg.Config.OAuthIssuer),
						TokenURL:         fmt.Sprintf("%s/protocol/openid-connect/token", cfg.Config.OAuthIssuer),
						Scopes: map[string]string{
							"openid":  "Scope for requesting OpenID token",
							"profile": "Scope for including user profile",
							"email":   "Scope for providing email information",
						},
					},
				},
			},
		}

		config.Servers = []*huma.Server{
			{URL: getApiUrl()},
		}
		api := humachi.New(r, config)
		registerControllers(api, []Controller{
			group.NewGroupController(&api),
			task.NewTaskController(&api),
			announcement.NewAnnouncementController(&api),
			user.NewUserController(&api),
			organization_management.NewOrganisationManagementController(&api),
		})

	})

	return router
}
