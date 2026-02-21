package test

import (
	"fmt"
	categoryDto "planeo/services/core/internal/infra/rest/api/v1/categories"
	"planeo/services/core/internal/test/utils"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCategoryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := utils.NewIntegrationTestEnvironment(t)
	testApi := env.Api

	t.Run("GET /v1/organizations/{organizationId}/categories", func(t *testing.T) {
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
					response := testApi.Get("/v1/organizations/1/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

					assert.Equal(t, 200, response.Code)
				})
			}
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Get("/v1/organizations/1/categories")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Get("/v1/organizations/1/categories", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not authorized for a given organization", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := testApi.Get("/v1/organizations/2/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})
	})

	//nolint:dupl
	t.Run("POST /v1/organizations/{organizationId}/categories", func(t *testing.T) {
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
					payload := categoryDto.CreateCategoryInputBody{
						Label:            "Test Category",
						LabelDescription: "Test description",
						Color:            "#000000",
					}
					response := testApi.Post("/v1/organizations/1/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), payload)

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
					payload:  `{"labelDescription": "Test description", "color": "#000000"}`,
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
					response := testApi.Post("/v1/organizations/1/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), tc.payload)

					assert.Equal(t, tc.expected, response.Code)
				})
			}
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Post("/v1/organizations/1/categories", categoryDto.CreateCategoryInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Post("/v1/organizations/1/categories", "Authorization: Bearer invalid", categoryDto.CreateCategoryInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not authorized for a given organization", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := testApi.Post("/v1/organizations/2/categories", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), categoryDto.CreateCategoryInputBody{})

			assert.Equal(t, 403, response.Code)
		})
	})

	//nolint:dupl
	t.Run("PUT /v1/organizations/{organizationId}/categories/{categoryId}", func(t *testing.T) {
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
					payload := categoryDto.UpdateCategoryInputBody{
						Label:            "Updated Category",
						LabelDescription: "Updated description",
						Color:            "#FFFFFF",
					}
					response := testApi.Put("/v1/organizations/1/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), payload)

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
					payload:  `{"labelDescription": "Test description", "color": "#000000"}`,
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
					response := testApi.Put("/v1/organizations/1/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), tc.payload)

					assert.Equal(t, tc.expected, response.Code)
				})
			}
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Put("/v1/organizations/1/categories/1", categoryDto.UpdateCategoryInputBody{})

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Put("/v1/organizations/1/categories/1", "Authorization: Bearer invalid", categoryDto.UpdateCategoryInputBody{})
			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not authorized for a given organization", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := testApi.Put("/v1/organizations/2/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken), categoryDto.UpdateCategoryInputBody{})

			assert.Equal(t, 403, response.Code)
		})
	})

	t.Run("DELETE /v1/organizations/{organizationId}/categories/{categoryId}", func(t *testing.T) {
		t.Run("should return 204 when user is admin", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := testApi.Delete("/v1/organizations/1/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 204, response.Code)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Delete("/v1/organizations/1/categories/1")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Delete("/v1/organizations/1/categories/1", "Authorization: Bearer invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 403 when user is not authorized for a given organization", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")

			if err != nil {
				t.Error(err)
			}

			assert.NotNil(t, session)
			response := testApi.Delete("/v1/organizations/2/categories/1", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 403, response.Code)
		})
	})
}
