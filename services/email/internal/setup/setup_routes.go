package setup

import (
	"context"
	"fmt"
	"planeo/libs/api"
	"planeo/libs/db"
	"planeo/libs/middlewares"
	"planeo/services/email/config"
	"planeo/services/email/internal"
	"planeo/services/email/internal/resources/settings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
)

func SetupRouter(config *config.ApplicationConfiguration, db *db.DBConnection) *chi.Mux {
	return api.SetupRouter(config, "/api", func(r chi.Router, humaAPI huma.API) {
		// Initialize controllers
		controllers := InitializeApplication(humaAPI, config, db)

		// Register controllers with middleware setup
		api.RegisterControllers(config, humaAPI, controllers, func(api huma.API) {
			jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", config.OauthIssuerUrl())
			api.UseMiddleware(middlewares.AuthMiddleware(api, jwksURL, config.OauthIssuerUrl()))
		})
	})
}

func InitializeApplication(humaAPi huma.API, config *config.ApplicationConfiguration, db *db.DBConnection) []api.Controller {

	settingsRepository := settings.NewSettingsRepository(db.DB)
	settingsService := settings.NewSettingsService(settingsRepository)
	settingsController := settings.NewSettingsController(humaAPi, config, settingsService)

	cronService := internal.NewCronService()
	cronService.Start()
	emailService := internal.NewEmailService(cronService)
	settings, err := settingsRepository.GetAllSettings(context.Background())
	if err != nil {
		panic(err)
	}
	emailService.StartFetching(settings)

	return []api.Controller{settingsController}
}
