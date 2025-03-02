package test

import (
	"context"
	"fmt"

	"planeo/services/core/internal/clients/keycloak"
	"planeo/services/core/internal/resources/user"
	"planeo/services/core/internal/resources/user/dto"
	"planeo/services/core/internal/resources/user/models"
	"planeo/services/core/internal/setup"
	"planeo/services/core/internal/test/utils"
	"planeo/services/core/pkg/db"
	jsonHelper "planeo/services/core/pkg/json"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
)

func TestUserRoleIntegration(t *testing.T) {

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

	t.Run("GET admin/roles ", func(t *testing.T) {

		t.Run("should return 200 when user is admin", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Get("/organizations/1/iam/roles", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var body struct{ Roles []models.Role }
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &body, true)
			assert.Greater(t, len(body.Roles), 0)
		})

		t.Run("should return 401 when user is not admin", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Get("/organizations/1/iam/roles", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := api.Get("/organizations/1/iam/roles")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := api.Get("/organizations/1/iam/roles", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Get("/organizations/3525/iam/roles", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})

	})

	t.Run("PUT admin/users/{userId}/roles", func(t *testing.T) {

		t.Run("should return 204 when user is admin", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			// pull roles first
			response := api.Get("/organizations/1/iam/roles", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))
			assert.Equal(t, 200, response.Code)
			var body struct{ Roles []models.Role }
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &body, true)
			assert.Greater(t, len(body.Roles), 0)

			// assign all roles
			response = api.Put("/organizations/1/iam/users/146b3857-090e-453d-b1e6-8cdfbb1a6dcb/roles", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), body.Roles)
			assert.Equal(t, 204, response.Code)
		})

		t.Run("should return 204 when user is admin and roles are empty", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Put("/organizations/1/iam/users/146b3857-090e-453d-b1e6-8cdfbb1a6dcb/roles", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), []dto.PutUserRoleInputBody{})
			assert.Equal(t, 204, response.Code)
		})

		t.Run("should return 401 when user is not admin", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Put("/organizations/1/iam/users/146b3857-090e-453d-b1e6-8cdfbb1a6dcb/roles", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), `{"roles":["admin"]}`)

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := api.Put("/organizations/1/iam/users/146b3857-090e-453d-b1e6-8cdfbb1a6dcb/roles", "", `{"roles":["admin"]}`)

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := api.Put("/organizations/1/iam/users/146b3857-090e-453d-b1e6-8cdfbb1a6dcb/roles", "Authorization: Bearer invalid", `{"roles":["admin"]}`)

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 for unknown organization to user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Put("/organizations/3525/iam/users/146b3857-090e-453d-b1e6-8cdfbb1a6dcb/roles", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), `{"roles":["admin"]}`)

			assert.Equal(t, 403, response.Code)
		})
	})

}
