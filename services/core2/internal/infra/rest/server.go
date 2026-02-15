package rest

import (
	"context"
	"fmt"
	"net/http"
	"planeo/libs/middlewares"
	"planeo/services/core2/internal/domain/category"
	"planeo/services/core2/internal/domain/organization"
	"planeo/services/core2/internal/domain/request"
	"planeo/services/core2/internal/domain/user"
	"planeo/services/core2/internal/infra/rest/api/v1/categories"
	"planeo/services/core2/internal/infra/rest/api/v1/organizations"
	"planeo/services/core2/internal/infra/rest/api/v1/requests"
	"planeo/services/core2/internal/infra/rest/api/v1/users"
	"strconv"
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
	OauthClientID    string
	EnableStackTrace bool
	AllowOrigins     []string
}

type Services struct {
	UserService         user.Service
	CategoryService     category.Service
	OrganizationService organization.Service
	RequestService      request.Service
}

type Server struct {
	Api    huma.API
	Router *chi.Mux
	Config Config
}

type Middleware = func(ctx huma.Context, next func(huma.Context))

func New(config Config, services Services) *Server {
	router := chi.NewRouter()
	router.Use(middlewares.LoggerMiddleware)
	router.Use(middleware.Recoverer)
	router.Use(middlewares.Cors(config.AllowOrigins))

	humaConfigV1 := huma.DefaultConfig(config.AppName, config.Version)
	humaConfigV1.Components.SecuritySchemes = getSecuritySchemes(config)
	humaConfigV1.OpenAPIPath = "/docs/v1/openapi"
	humaConfigV1.DocsPath = "/docs/v1/"
	humaConfigV1.Servers = []*huma.Server{
		{URL: getApiUrl(config)},
	}

	apiRouter := chi.NewRouter()
	router.Mount("/api", apiRouter)
	api := humachi.New(apiRouter, humaConfigV1)

	initRoutes(api, config, services)

	return &Server{
		Api:    api,
		Router: router,
		Config: config,
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

func initRoutes(api huma.API, config Config, services Services) {
	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", config.OauthIssuerUrl)
	appMiddlewares := []Middleware{
		middlewares.AuthMiddleware(api, jwksURL, config.OauthIssuerUrl),
		middlewares.OrganizationCheckMiddleware(api, func(organizationId string) (string, error) {
			id, err := strconv.Atoi(organizationId)
			if err != nil {
				return "", err
			}

			organization, err := services.OrganizationService.GetOrganizationById(context.Background(), id)
			if err != nil {
				return "", err
			}

			return organization.IAMOrganizationID, nil
		}),
	}
	permissions := middlewares.NewPermissionMiddlewareConfig(api, config.OauthIssuerUrl, config.OauthClientID)

	// register status endpoint
	registerStatusEndpoint(api)

	// register application middlewares
	api.UseMiddleware(appMiddlewares...)

	// create handler
	categoryHandler := categories.NewCategoriesHandler(services.CategoryService)
	organizationHandler := organizations.NewOrganizationHandler(services.OrganizationService)
	requestHandler := requests.NewRequestHandler(services.RequestService)
	userHandler := users.NewUserHandler(services.UserService)

	// register routes
	categoryHandler.RegisterRoutes(api, *permissions)
	organizationHandler.RegisterRoutes(api, *permissions)
	requestHandler.RegisterRoutes(api, *permissions)
	userHandler.RegisterRoutes(api, *permissions)
}
