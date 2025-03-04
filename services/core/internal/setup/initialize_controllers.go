package setup

import (
	"planeo/libs/db"
	"planeo/services/core/config"
	"planeo/services/core/internal/clients/keycloak"
	"planeo/services/core/internal/resources/announcement"
	"planeo/services/core/internal/resources/category"
	"planeo/services/core/internal/resources/group"
	"planeo/services/core/internal/resources/request"
	"planeo/services/core/internal/resources/user"

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

	// Category controller
	categoryRepository := category.NewCategoryRepository(db.DB)
	categoryService := category.NewCategoryService(categoryRepository)
	categoryController := category.NewCategoryController(api, config, categoryService)

	return []Controller{groupController, requestController, announcementController, userController, categoryController}
}
