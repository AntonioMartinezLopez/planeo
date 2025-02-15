package setup

import (
	"planeo/api/config"
	"planeo/api/internal/clients/keycloak"
	"planeo/api/internal/resources/announcement"
	"planeo/api/internal/resources/group"
	"planeo/api/internal/resources/request"
	"planeo/api/internal/resources/user"
	"planeo/api/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

func InitializeControllers(api huma.API, config *config.ApplicationConfiguration, db *db.DBConnection) []Controller {

	// Prepare dependencies
	keycloakAdminClient := keycloak.NewKeycloakAdminClient(*config)

	// Group controller
	groupController := group.NewGroupController(api, config)

	// Request controller
	requestRepository := request.NewRequestRepository(db.DB)
	requestService := request.NewRequestService(requestRepository)
	requestController := request.NewRequestController(api, config, requestService)

	// Announcement controller
	announcementController := announcement.NewAnnouncementController(api, config)

	// User controller
	userRepository := user.NewUserRepository(db.DB)
	keylcoakService := user.NewKeycloakService(keycloakAdminClient, config)
	userService := user.NewUserService(userRepository, keylcoakService)
	userController := user.NewUserController(api, config, userService)

	return []Controller{groupController, requestController, announcementController, userController}
}
