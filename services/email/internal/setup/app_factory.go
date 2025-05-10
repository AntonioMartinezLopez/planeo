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
)

type Repositories struct {
	SettingsRepository *settings.SettingsRepository
}

type Services struct {
	SettingsService *settings.SettingsService
	CronService     *internal.CronService
	IMAPService     *internal.IMAPService
	EmailService    *internal.EmailService
}

type Controllers struct {
	SettingsController *settings.SettingsController
}

type Application struct {
	Repositories Repositories
	Services     Services
	Controllers  Controllers
	API          *api.API
}

type ApplicationFactory struct{}

// NewApplicationFactory creates a new instance of ApplicationFactory
func NewApplicationFactory() *ApplicationFactory {
	return &ApplicationFactory{}
}

func (f *ApplicationFactory) CreateApplication(config *config.ApplicationConfiguration, db *db.DBConnection, eventService *events.EventService) *Application {
	// Initialize repositories
	settingsRepository := settings.NewSettingsRepository(db.DB)

	// Initialize services
	cronService := internal.NewCronService()
	cronService.Start()
	imapService := internal.NewIMAPService()
	emailService := internal.NewEmailService(cronService, imapService, eventService)
	settingsService := settings.NewSettingsService(settingsRepository, emailService)

	// Initialize API
	huma := api.NewHumaAPI(config, "Planeo E-Mail Service", "0.0.1", "/api")

	// Initialize controllers
	settingsController := settings.NewSettingsController(huma.Api, config, settingsService)

	// Register controllers with middleware setup
	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", config.OauthIssuerUrl())
	middlewares := []api.Middleware{
		middlewares.AuthMiddleware(huma.Api, jwksURL, config.OauthIssuerUrl()),
	}
	controllers := []api.Controller{
		settingsController,
	}

	api.InitializeControllers(huma.Api, middlewares, controllers)

	// Create and return the application instance
	return &Application{
		Repositories: Repositories{
			SettingsRepository: settingsRepository,
		},
		Services: Services{
			SettingsService: settingsService,
			CronService:     cronService,
			IMAPService:     imapService,
			EmailService:    emailService,
		},
		Controllers: Controllers{
			SettingsController: settingsController,
		},
		API: huma,
	}
}
