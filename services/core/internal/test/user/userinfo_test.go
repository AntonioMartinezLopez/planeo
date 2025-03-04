package test

import (
	"context"
	"fmt"

	"planeo/libs/db"
	jsonHelper "planeo/libs/json"
	"planeo/services/core/internal/clients/keycloak"
	"planeo/services/core/internal/resources/user"
	"planeo/services/core/internal/resources/user/models"
	"planeo/services/core/internal/setup"
	"planeo/services/core/internal/test/utils"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
)

func TestUserinfoIntegration(t *testing.T) {

	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := utils.NewIntegrationTestEnvironment(t)
	db := db.InitializeDatabaseConnection(context.Background(), env.Configuration.DatabaseConfig())
	_, api := humatest.New(t)

	// setup user controller
	keycloakAdminClient := keycloak.NewKeycloakAdminClient(*env.Configuration)
	userRepository := user.NewUserRepository(db.DB)
	keylcoakService := user.NewKeycloakService(keycloakAdminClient, env.Configuration)
	userService := user.NewUserService(userRepository, keylcoakService)
	userController := user.NewUserController(api, env.Configuration, userService)

	// Register controllers
	setup.RegisterControllers(env.Configuration, api, db, []setup.Controller{userController})

	t.Run("GET /users ", func(t *testing.T) {

		t.Run("should return 200 when user is admin", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			println("session", session.AccessToken)

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Get("/organizations/1/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var body struct{ Users []models.BasicUserInformation }
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &body, true)
			assert.Greater(t, len(body.Users), 0)
		})

		t.Run("should return 200 when user has user role", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Get("/organizations/1/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var body struct{ Users []models.BasicUserInformation }
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &body, true)
			assert.Greater(t, len(body.Users), 0)
		})

		t.Run("should return 200 when user has planner role", func(t *testing.T) {
			session, err := env.GetUserSession("planner", "planner")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Get("/organizations/1/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var body struct{ Users []models.BasicUserInformation }
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &body, true)
			assert.Greater(t, len(body.Users), 0)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := api.Get("/organizations/1/users")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := api.Get("/organizations/1/users", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Get("/organizations/2/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})
	})

}
