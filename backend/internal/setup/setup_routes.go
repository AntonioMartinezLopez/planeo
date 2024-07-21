package setup

import (
	"context"
	"net/http"
	"os"
	"strings"

	cfg "planeo/api/config"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/task"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func registerRoutes(api huma.API) {

	jwksURL := strings.Join([]string{os.Getenv("OAUTH_ISSUER"), "/.well-known/jwks.json"}, "")
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

	// protect routes with middleware

	// Add new routes
	task.TaskRouter(api)

	// rootRouter.Route("/api", func(r chi.Router) {
	// 	r.Use(middlewares.JwtValidator)
	// 	// Add new sub routers
	// 	task.TaskRouter(r)
	// 	announcement.AnnouncementRouter(r)
	// })

	// // GreetingOutput represents the greeting operation response.
	// type GreetingOutput struct {
	// 	Body struct {
	// 		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	// 	}
	// }

	// // Register GET /greeting/{name} handler.
	// huma.Get(api, "/greeting/{name}", func(ctx context.Context, input *struct {
	// 	Name string `path:"name" maxLength:"35" example:"world" doc:"Name to greet"`
	// }) (*GreetingOutput, error) {
	// 	resp := &GreetingOutput{}
	// 	resp.Body.Message = fmt.Sprintf("Hello, %s!", input.Name)
	// 	return resp, nil
	// })

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

		config := huma.DefaultConfig("My API", "1.0.0")
		config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
			// // Example Authorization Code flow.
			"myAuth": {
				Type: "oauth2",
				Flows: &huma.OAuthFlows{
					AuthorizationCode: &huma.OAuthFlow{
						AuthorizationURL: "https://dev-3jftnb3rml6xpid5.eu.auth0.com/oauth/authorize",
						TokenURL:         "https://dev-3jftnb3rml6xpid5.eu.auth0.com/oauth/token",

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
		registerRoutes(api)

	})

	return router
}
