package test

import (
	"context"
	"fmt"
	"slices"

	"planeo/api/internal/clients/keycloak"
	"planeo/api/internal/resources/user"
	"planeo/api/internal/resources/user/dto"
	"planeo/api/internal/resources/user/models"
	"planeo/api/internal/setup"
	"planeo/api/internal/test/utils"
	"planeo/api/pkg/db"
	jsonHelper "planeo/api/pkg/json"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
)

func TestUserIntegration(t *testing.T) {

	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := utils.NewIntegrationTestEnvironment(t)

	// Initialize database connection
	t.Log("Initializing database connection")
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

	t.Run("GET /admin/users", func(t *testing.T) {

		t.Run("should return 200 and all users", func(t *testing.T) {

			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Get("/organizations/1/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var body struct{ Users []models.User }
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &body, true)
			assert.Greater(t, len(body.Users), 0)
		})

		t.Run("should return 401 when no token is provided", func(t *testing.T) {
			response := api.Get("/organizations/1/iam/users", "")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not admin", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Get("/organizations/1/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user access an organization he does not belong to", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Get("/organizations/3525/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})
	})

	t.Run("GET /admin/users/{userId}", func(t *testing.T) {

		t.Run("should return 200 and user", func(t *testing.T) {

			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Get("/organizations/1/iam/users/d7eddb93-254e-4482-9a53-f31a5975dd1d", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var body struct{ User models.User }
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &body, true)
			assert.NotNil(t, body.User)
			assert.Equal(t, "d7eddb93-254e-4482-9a53-f31a5975dd1d", body.User.Id)
			assert.Equal(t, "user@local.de", body.User.Username)
		})

		t.Run("should return 404 when user does not exist", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Get("/organizations/1/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 404, response.Code)

			var body struct{ User models.User }
			err = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &body, true)
			assert.NotNil(t, err)
		})

		t.Run("should return 401 when no token is provided", func(t *testing.T) {
			response := api.Get("/organizations/1/iam/users/1", "")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not admin", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Get("/organizations/1/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user access an organization he does not belong to", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Get("/organizations/3525/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should return 404 when user id can not be found in given organzation", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Get("/organizations/1/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 404, response.Code)
		})
	})

	t.Run("POST /admin/users", func(t *testing.T) {

		t.Run("should return 201 and create user", func(t *testing.T) {

			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			body := dto.CreateUserInputBody{
				FirstName: "John",
				LastName:  "Doe",
				Email:     "John.Doe@local.de",
				Password:  "password123",
			}

			response := api.Post("/organizations/1/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), body)

			assert.Equal(t, 201, response.Code)

		})

		t.Run("should return 401 when no token is provided", func(t *testing.T) {
			response := api.Post("/organizations/1/iam/users", "", dto.CreateUserInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not admin", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Post("/organizations/1/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.CreateUserInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user access an organization he does not belong to", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Post("/organizations/3525/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.CreateUserInputBody{})

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should return 400 when user already exists", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			body := dto.CreateUserInputBody{
				FirstName: "John",
				LastName:  "Doe",
				Email:     "user@local.de",
				Password:  "password123",
			}

			response := api.Post("/organizations/1/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), body)

			assert.Equal(t, 500, response.Code)
		})

		t.Run("should return 422 when input body is invalid", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			body := []map[string]interface{}{
				{"firstName": "John", "lastName": "Doe", "email": 12, "password": "password123"},
				{"firstName": 123, "lastName": "Doe", "email": "John.Doe@local.de", "password": "password123"},
				{"firstName": "John", "lastName": 123, "email": "John.Doe@local.de", "password": "password123"},
				{"firstName": "John", "lastName": "Doe", "email": "John.Doe@local.de", "password": 123},
			}

			for _, b := range body {
				response := api.Post("/organizations/1/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), b)
				assert.Equal(t, 422, response.Code)
			}

		})
	})

	t.Run("PUT /admin/users/{userId}", func(t *testing.T) {

		t.Run("should return 200 and update user", func(t *testing.T) {

			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			// get user
			response := api.Get("/organizations/1/iam/users/146b3857-090e-453d-b1e6-8cdfbb1a6dcb", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			var body struct{ User models.User }
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &body, true)

			updatePayload := dto.UpdateUserInputBody{
				FirstName:       "Jane",
				LastName:        "Doe",
				Email:           body.User.Email,
				Username:        body.User.Username,
				Totp:            body.User.Totp,
				Enabled:         body.User.Enabled,
				EmailVerified:   body.User.EmailVerified,
				RequiredActions: body.User.RequiredActions,
			}

			response = api.Put(fmt.Sprintf("/organizations/1/iam/users/%s", body.User.Id), fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), updatePayload)

			assert.Equal(t, 200, response.Code)
		})

		t.Run("should return 401 when no token is provided", func(t *testing.T) {
			response := api.Put("/organizations/1/iam/users/1", "", dto.UpdateUserInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not admin", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Put("/organizations/1/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.UpdateUserInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user access an organization he does not belong to", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Put("/organizations/3525/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.UpdateUserInputBody{})

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should return 404 when user does not exist in organization", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			updatePayload := dto.UpdateUserInputBody{
				FirstName:       "Jane",
				LastName:        "Doe",
				Email:           "test",
				Username:        "test",
				Totp:            false,
				Enabled:         true,
				EmailVerified:   true,
				RequiredActions: []models.RequiredAction{},
			}

			response := api.Put("/organizations/1/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), updatePayload)

			assert.Equal(t, 404, response.Code)
		})

		t.Run("should return 422 when input body is invalid", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			body := []map[string]interface{}{
				{"firstName": 12, "lastName": "Doe", "email": "test@mail.de", "username": "test", "totp": false, "enabled": true, "emailVerified": true, "requiredActions": []string{}},
				{"firstName": "John", "lastName": "Doe", "email": 12, "username": "test", "totp": false, "enabled": true, "emailVerified": true, "requiredActions": []string{}},
				{"firstName": "John", "lastName": "Doe", "email": "test@mail.de", "username": 12, "totp": false, "enabled": true, "emailVerified": true, "requiredActions": []string{}},
				{"firstName": "John", "lastName": "Doe", "email": "test@mail.de", "username": "test", "totp": "test", "enabled": true, "emailVerified": true, "requiredActions": []string{}},
				{"firstName": "John", "lastName": "Doe", "email": "test@mail.de", "username": "test", "totp": false, "enabled": "wrong", "emailVerified": true, "requiredActions": []string{}},
				{"firstName": "John", "lastName": "Doe", "email": "test@mail.de", "username": "test", "totp": false, "enabled": "wrong", "emailVerified": 12, "requiredActions": []string{}},
				{"firstName": "John", "lastName": "Doe", "email": "test@mail.de", "username": "test", "totp": false, "enabled": "wrong", "emailVerified": 12, "requiredActions": []string{"test"}},
				{"firstName": "John", "lastName": "Doe", "email": "test@mail.de", "username": "test", "totp": false, "enabled": "wrong", "emailVerified": true, "requiredActions": "test"},
			}

			for _, b := range body {
				response := api.Put("/organizations/1/iam/users/146b3857-090e-453d-b1e6-8cdfbb1a6dcb", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), b)
				assert.Equal(t, 422, response.Code)
			}
		})

	})

	t.Run("DELETE /admin/users/{userId}", func(t *testing.T) {

		t.Run("should return 204 and delete user", func(t *testing.T) {

			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			body := dto.CreateUserInputBody{
				FirstName: "John",
				LastName:  "Test",
				Email:     "john.test@local.de",
				Password:  "password123",
			}

			response := api.Post("/organizations/1/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), body)

			assert.Equal(t, 201, response.Code)

			response = api.Get("/organizations/1/iam/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var users struct{ Users []models.User }
			jsonHelper.DecodeJSONAndValidate(response.Result().Body, &users, true)

			index := slices.IndexFunc(users.Users, func(u models.User) bool {
				return u.Email == "john.test@local.de"
			})

			assert.NotEqual(t, -1, index)

			userToDelete := users.Users[index]

			response = api.Delete(fmt.Sprintf("/organizations/1/iam/users/%s", userToDelete.Id), fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))
			assert.Equal(t, 200, response.Code)
		})

		t.Run("should return 401 when no token is provided", func(t *testing.T) {
			response := api.Delete("/organizations/1/iam/users/1", "")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not admin", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Delete("/organizations/1/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user access an organization he does not belong to", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Delete("/organizations/3525/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})

		t.Run("should return 404 when user does not exist in organization", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)

			response := api.Delete("/organizations/1/iam/users/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 404, response.Code)
		})
	})
}
