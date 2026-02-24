package setup

import (
	"context"
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
	CronService     *internal.CronService       // Deprecated - will be removed
	IMAPService     *internal.IMAPService
	EmailService    *internal.EmailService      // Deprecated - will be removed
	JobPublisher    *internal.JobPublisher      // New: Publishes jobs to NATS
	JobWorker       *internal.JobWorker         // New: Processes jobs from NATS
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

func (f *ApplicationFactory) CreateApplication(config *config.ApplicationConfiguration, db *db.DBConnection, eventService events.EventServiceInterface) *Application {
	// Initialize repositories
	settingsRepository := settings.NewSettingsRepository(db.DB)

	// Initialize core services
	imapService := internal.NewIMAPService()

	// Initialize based on mode
	var jobPublisher *internal.JobPublisher
	var jobWorker *internal.JobWorker
	var settingsService *settings.SettingsService

	if config.IsPublisher() {
		// Publisher mode: Schedule jobs and publish to NATS
		jobPublisher = internal.NewJobPublisher(eventService)
		schedulerService := internal.NewEmailSchedulerService(jobPublisher, imapService)
		settingsService = settings.NewSettingsService(settingsRepository, schedulerService)
	}

	if config.IsWorker() {
		// Worker mode: Process jobs from NATS
		jobWorker = internal.NewJobWorker(imapService, eventService)
		ctx := context.Background()
		if err := jobWorker.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start job worker: %v", err))
		}
	}

	// If publisher mode is not enabled, create a no-op settings service
	if !config.IsPublisher() {
		// In worker-only mode, settings service doesn't manage scheduling
		// You might want to create a simplified version or skip API endpoints
		settingsService = nil // Handle this appropriately in your API
	}

	// Deprecated: Keep old services for backward compatibility during migration
	var cronService *internal.CronService
	var emailService *internal.EmailService
	if config.Mode == "both" || config.Mode == "" {
		// Legacy mode - for backward compatibility
		cronService = internal.NewCronService()
		cronService.Start()
		emailService = internal.NewEmailService(cronService, imapService, eventService)
		if settingsService == nil {
			settingsService = settings.NewSettingsService(settingsRepository, emailService)
		}
	}

	// Initialize API
	huma := api.NewHumaAPI(config, "Planeo E-Mail Service", "0.0.1", "/api")

	// Initialize controllers (only in publisher mode or legacy mode)
	var settingsController *settings.SettingsController
	if settingsService != nil {
		settingsController = settings.NewSettingsController(huma.Api, config, settingsService)

		// Register controllers with middleware setup
		jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", config.OauthIssuerUrl())
		middlewares := []api.Middleware{
			middlewares.AuthMiddleware(huma.Api, jwksURL, config.OauthIssuerUrl()),
		}
		controllers := []api.Controller{
			settingsController,
		}

		api.InitializeControllers(huma.Api, middlewares, controllers)
	}

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
			JobPublisher:    jobPublisher,
			JobWorker:       jobWorker,
		},
		Controllers: Controllers{
			SettingsController: settingsController,
		},
		API: huma,
	}
}
