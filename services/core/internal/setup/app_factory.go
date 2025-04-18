package setup

import (
	"context"
	"fmt"
	"planeo/libs/api"
	"planeo/libs/db"
	"planeo/libs/events"
	"planeo/libs/middlewares"
	"planeo/services/core/config"
	"planeo/services/core/internal/clients/keycloak"
	coreEvents "planeo/services/core/internal/events"
	"planeo/services/core/internal/resources/category"
	"planeo/services/core/internal/resources/organization"
	"planeo/services/core/internal/resources/request"
	"planeo/services/core/internal/resources/user"
)

type Repositories struct {
	RequestRepository  *request.RequestRepository
	UserRepository     *user.UserRepository
	CategoryRepository *category.CategoryRepository
	KeycloakService    *user.KeycloakService
}

type Services struct {
	RequestService  *request.RequestService
	UserService     *user.UserService
	CategoryService *category.CategoryService
	KeyCloakService *user.KeycloakService
	EventService    *coreEvents.EventService
}

type Controllers struct {
	RequestController  *request.RequestController
	UserController     *user.UserController
	CategoryController *category.CategoryController
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

func (f *ApplicationFactory) CreateApplication(config *config.ApplicationConfiguration, db *db.DBConnection, natsClient *events.NatsConnector) *Application {
	// Initialize repositories
	requestRepository := request.NewRequestRepository(db.DB)
	userRepository := user.NewUserRepository(db.DB)
	categoryRepository := category.NewCategoryRepository(db.DB)

	// Initialize services
	keycloakAdminClient := keycloak.NewKeycloakAdminClient(*config)
	keycloakService := user.NewKeycloakService(keycloakAdminClient, config)
	requestService := request.NewRequestService(requestRepository)
	userService := user.NewUserService(userRepository, keycloakService)
	categoryService := category.NewCategoryService(categoryRepository)
	eventService := coreEvents.NewEventService(natsClient)

	// Initialize API
	huma := api.NewHumaAPI(config, "Planeo Core", "0.0.1", "/api")

	// Initialize controllers
	requestController := request.NewRequestController(huma.Api, config, requestService)
	userController := user.NewUserController(huma.Api, config, userService)
	categoryController := category.NewCategoryController(huma.Api, config, categoryService)

	// Register controllers with middleware setup
	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", config.OauthIssuerUrl())
	middlewares := []api.Middleware{
		middlewares.AuthMiddleware(huma.Api, jwksURL, config.OauthIssuerUrl()),
		middlewares.OrganizationCheckMiddleware(huma.Api, func(organizationId string) (string, error) {
			return organization.GetOrganizationIamById(db.DB, organizationId)
		})}
	controllers := []api.Controller{
		requestController,
		userController,
		categoryController,
	}

	api.InitializeControllers(huma.Api, middlewares, controllers)

	// Start listening to events set in service
	err := eventService.InitializeEvents(context.Background(), coreEvents.Services{
		RequestService: requestService})
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize events: %v", err))
	}

	return &Application{
		Repositories: Repositories{
			RequestRepository:  requestRepository,
			UserRepository:     userRepository,
			CategoryRepository: categoryRepository,
		},
		Services: Services{
			RequestService:  requestService,
			UserService:     userService,
			CategoryService: categoryService,
			KeyCloakService: keycloakService,
			EventService:    eventService,
		},
		Controllers: Controllers{
			RequestController:  requestController,
			UserController:     userController,
			CategoryController: categoryController,
		},
		API: huma,
	}
}
