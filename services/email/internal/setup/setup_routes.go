package setup

import (
	"fmt"
	"planeo/libs/api"
	"planeo/libs/db"
	"planeo/libs/middlewares"
	"planeo/services/email/config"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
)

func SetupRouter(config *config.ApplicationConfiguration, db *db.DBConnection) *chi.Mux {
	return api.SetupRouter(config, "/api", func(r chi.Router, humaAPI huma.API) {
		// Initialize controllers
		controllers := InitializeControllers(humaAPI, config, db)

		// Register controllers with middleware setup
		api.RegisterControllers(config, humaAPI, controllers, func(api huma.API) {
			jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", config.OauthIssuerUrl())
			api.UseMiddleware(middlewares.AuthMiddleware(api, jwksURL, config.OauthIssuerUrl()))
		})
	})
}

func InitializeControllers(humaAPi huma.API, config *config.ApplicationConfiguration, db *db.DBConnection) []api.Controller {
	return []api.Controller{}
}
