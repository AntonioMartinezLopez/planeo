package test

import (
	"context"
	"fmt"
	"planeo/api/internal/resources/category"
	"planeo/api/internal/resources/category/dto"
	"planeo/api/internal/setup"
	"planeo/api/internal/test/utils"
	"planeo/api/pkg/db"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
)

func TestCategoryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := utils.NewIntegrationTestEnvironment(t)
	db := db.InitializeDatabaseConnection(context.Background(), env.Configuration.DatabaseConfig())
	_, api := humatest.New(t)

	// setup category controller
	categoryRepository := category.NewCategoryRepository(db.DB)
	categoryService := category.NewCategoryService(categoryRepository)
	categoryController := category.NewCategoryController(api, env.Configuration, categoryService)

	// Register controllers
	setup.RegisterControllers(env.Configuration, api, db, []setup.Controller{categoryController})

	t.Run("GET /categories", func(t *testing.T) {
		t.Run("Test authorization for category access", func(t *testing.T) {
			testCases := []struct {
				name     string
				username string
				password string
			}{
				{"should return 200 when user is admin", "admin", "admin"},
				{"should return 200 when user is planner", "planner", "planner"},
				{"should return 200 when user is user", "user", "user"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession(tc.username, tc.password)

					if err != nil {
						t.Error(err)
					}

					assert.NotNil(t, session)
					response := api.Get("/organizations/1/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

					assert.Equal(t, 200, response.Code)
				})
			}
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := api.Get("/organizations/1/categories")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := api.Get("/organizations/1/categories", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not authorized for a given organization", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Get("/organizations/2/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})
	})

	t.Run("POST /categories", func(t *testing.T) {
		t.Run("Test authorization for category creation", func(t *testing.T) {
			testCases := []struct {
				name     string
				username string
				password string
			}{
				{"should return 201 when user is admin", "admin", "admin"},
				{"should return 201 when user is planner", "planner", "planner"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession(tc.username, tc.password)

					if err != nil {
						t.Error(err)
					}

					assert.NotNil(t, session)
					payload := dto.CreateCategoryInputBody{
						Label:            "Test Category",
						LabelDescription: "Test description",
						Color:            "#000000",
					}
					response := api.Post("/organizations/1/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), payload)

					assert.Equal(t, 201, response.Code)
				})
			}
		})

		t.Run("should return 400 with missing or invalid required fields", func(t *testing.T) {
			testCases := []struct {
				name     string
				payload  string
				expected int
			}{
				{
					name:     "missing label",
					payload:  `{"labelDescription": "Test description", "Cclor": "#000000"}`,
					expected: 400,
				},
				{
					name:     "missing color",
					payload:  `{"label": "Test Category", "labelDescription": "Test description"}`,
					expected: 400,
				},
				{
					name:     "missing label description",
					payload:  `{"label": "Test Category", "color": "#000000"}`,
					expected: 400,
				},
				{
					name:     "wrong type for label",
					payload:  `{"label": 123, "labelDescription": "Test description", "color": "#000000"}`,
					expected: 400,
				},
				{
					name:     "wrong type for label description",
					payload:  `{"label": "Test Category", "labelDescription": 123, "color": "#000000"}`,
					expected: 400,
				},
				{
					name:     "wrong type for color",
					payload:  `{"label": "Test Category", "labelDescription": "Test description", "color": 123}`,
					expected: 400,
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession("admin", "admin")

					if err != nil {
						t.Error(err)
					}

					assert.NotNil(t, session)
					response := api.Post("/organizations/1/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), tc.payload)

					assert.Equal(t, tc.expected, response.Code)
				})
			}
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := api.Post("/organizations/1/categories", dto.CreateCategoryInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := api.Post("/organizations/1/categories", "Authorization: Bearer invalid", dto.CreateCategoryInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not authorized for a given organization", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Post("/organizations/2/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.CreateCategoryInputBody{})

			assert.Equal(t, 403, response.Code)
		})
	})

	t.Run("PUT /categories/{categoryId}", func(t *testing.T) {
		t.Run("Test authorization for category update", func(t *testing.T) {
			testCases := []struct {
				name     string
				username string
				password string
			}{
				{"should return 204 when user is admin", "admin", "admin"},
				{"should return 204 when user is planner", "planner", "planner"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession(tc.username, tc.password)

					if err != nil {
						t.Error(err)
					}

					assert.NotNil(t, session)
					payload := dto.UpdateCategoryInputBody{
						Label:            "Updated Category",
						LabelDescription: "Updated description",
						Color:            "#FFFFFF",
					}
					response := api.Put("/organizations/1/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), payload)

					assert.Equal(t, 204, response.Code)
				})
			}
		})

		t.Run("should return 400 with missing or invalid required fields", func(t *testing.T) {
			testCases := []struct {
				name     string
				payload  string
				expected int
			}{
				{
					name:     "missing label",
					payload:  `{"labelDescription": "Test description", "Cclor": "#000000"}`,
					expected: 400,
				},
				{
					name:     "missing color",
					payload:  `{"label": "Test Category", "labelDescription": "Test description"}`,
					expected: 400,
				},
				{
					name:     "missing label description",
					payload:  `{"label": "Test Category", "color": "#000000"}`,
					expected: 400,
				},
				{
					name:     "wrong type for label",
					payload:  `{"label": 123, "labelDescription": "Test description", "color": "#000000"}`,
					expected: 400,
				},
				{
					name:     "wrong type for label description",
					payload:  `{"label": "Test Category", "labelDescription": 123, "color": "#000000"}`,
					expected: 400,
				},
				{
					name:     "wrong type for color",
					payload:  `{"label": "Test Category", "labelDescription": "Test description", "color": 123}`,
					expected: 400,
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession("admin", "admin")

					if err != nil {
						t.Error(err)
					}

					assert.NotNil(t, session)
					response := api.Put("/organizations/1/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), tc.payload)

					assert.Equal(t, tc.expected, response.Code)
				})
			}
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := api.Put("/organizations/1/categories/1", dto.UpdateCategoryInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := api.Put("/organizations/1/categories/1", "Authorization: Bearer invalid", dto.UpdateCategoryInputBody{})
			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not authorized for a given organization", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Put("/organizations/2/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), dto.UpdateCategoryInputBody{})

			assert.Equal(t, 403, response.Code)
		})
	})

	t.Run("DELETE /categories/{categoryId}", func(t *testing.T) {
		t.Run("should return 204 when user is admin", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Delete("/organizations/1/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 204, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := api.Delete("/organizations/1/categories/1")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := api.Delete("/organizations/1/categories/1", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not authorized for a given organization", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := api.Delete("/organizations/2/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})
	})
}
