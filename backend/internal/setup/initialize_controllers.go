package setup

import (
	cfg "planeo/api/config"
	"planeo/api/internal/clients/keycloak"
	"planeo/api/internal/resources/announcement"
	"planeo/api/internal/resources/group"
	"planeo/api/internal/resources/task"
	"planeo/api/internal/resources/user"
	"planeo/api/pkg/db"

	"github.com/danielgtaylor/huma/v2"
)

func InitializeControllers(api *huma.API) []Controller {

	// Prepare dependencies
	database := db.GetDatabaseConnection()
	keycloakAdminClient := keycloak.NewKeycloakAdminClient(*cfg.Config)

	// Group controller
	groupController := group.NewGroupController(api)

	// Task controller
	taskController := task.NewTaskController(api)

	// Announcement controller
	announcementController := announcement.NewAnnouncementController(api)

	// User controller
	userRepository := user.NewUserRepository(database)
	keylcoakService := user.NewKeycloakService(keycloakAdminClient)
	userService := user.NewUserService(userRepository, keylcoakService)
	userController := user.NewUserController(api, userService)

	return []Controller{groupController, taskController, announcementController, userController}
}
