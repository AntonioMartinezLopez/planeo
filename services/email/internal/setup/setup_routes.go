package setup

import (
	"fmt"
	"planeo/libs/api"
	"planeo/libs/db"
	"planeo/libs/events"
	"planeo/libs/middlewares"
	"planeo/services/email/config"
	"planeo/services/email/internal"
	"planeo/services/email/internal/resources/settings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
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

	cronService := internal.NewCronService()
	cronService.Start()
	imapService := internal.NewIMAPService()

	eventService, err := events.NewNatsConnector(config.NatsUrl, []nats.Option{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to NATS")
		panic(err)
	}

	emailService := internal.NewEmailService(cronService, imapService, eventService)
	settingsRepository := settings.NewSettingsRepository(db.DB)
	settingsService := settings.NewSettingsService(settingsRepository, emailService)
	settingsController := settings.NewSettingsController(humaAPi, config, settingsService)

	return []api.Controller{settingsController}
}
