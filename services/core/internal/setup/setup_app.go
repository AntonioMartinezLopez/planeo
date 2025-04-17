package setup

import (
	"context"
	"fmt"

	"planeo/libs/api"
	"planeo/libs/db"
	eventsInit "planeo/libs/events"
	"planeo/libs/middlewares"
	"planeo/services/core/config"
	"planeo/services/core/internal/clients/keycloak"
	"planeo/services/core/internal/events"
	"planeo/services/core/internal/resources/category"
	"planeo/services/core/internal/resources/organization"
	"planeo/services/core/internal/resources/request"
	"planeo/services/core/internal/resources/user"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
)

func SetupRouter(config *config.ApplicationConfiguration, db *db.DBConnection) *chi.Mux {
	return api.SetupRouter(config, "/api", func(r chi.Router, humaAPI huma.API) {
		// Initialize controllers
		controllers := InitializeAppComponents(humaAPI, config, db)

		// Register controllers with middleware setup
		api.RegisterControllers(config, humaAPI, controllers, func(api huma.API) {
			jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", config.OauthIssuerUrl())
			api.UseMiddleware(middlewares.AuthMiddleware(api, jwksURL, config.OauthIssuerUrl()))
			api.UseMiddleware(middlewares.OrganizationCheckMiddleware(api, func(organizationId string) (string, error) {
				return organization.GetOrganizationIamById(db.DB, organizationId)
			}))
		})
	})
}

func InitializeAppComponents(humaApi huma.API, config *config.ApplicationConfiguration, db *db.DBConnection) []api.Controller {
	// Prepare dependencies
	keycloakAdminClient := keycloak.NewKeycloakAdminClient(*config)
	natsConnector, err := eventsInit.NewNatsConnector(config.NatsUrl, []nats.Option{})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to NATS: %v", err))
	}
	eventService := events.NewEventService(natsConnector)

	// Request controller
	requestRepository := request.NewRequestRepository(db.DB)
	requestService := request.NewRequestService(requestRepository)
	requestController := request.NewRequestController(humaApi, config, requestService)

	// User controller
	userRepository := user.NewUserRepository(db.DB)
	keylcoakService := user.NewKeycloakService(keycloakAdminClient, config)
	userService := user.NewUserService(userRepository, keylcoakService)
	userController := user.NewUserController(humaApi, config, userService)

	// Category controller
	categoryRepository := category.NewCategoryRepository(db.DB)
	categoryService := category.NewCategoryService(categoryRepository)
	categoryController := category.NewCategoryController(humaApi, config, categoryService)

	// Initialize events
	err = eventService.InitializeEvents(context.Background(), events.Services{
		RequestService: requestService})
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize events: %v", err))
	}

	return []api.Controller{requestController, userController, categoryController}
}
