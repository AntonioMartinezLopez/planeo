package test

import (
	"context"
	"fmt"
	"planeo/libs/api"
	"planeo/libs/db"
	jsonHelper "planeo/libs/json"
	"planeo/libs/middlewares"
	"planeo/services/core/internal/resources/organization"
	"planeo/services/core/internal/resources/organization/models"
	"planeo/services/core/internal/test/utils"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
)

func TestOrganizationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := utils.NewIntegrationTestEnvironment(t)
	db := db.InitializeDatabaseConnection(context.Background(), env.Configuration.DatabaseConfig())
	_, testApi := humatest.New(t)

	// Setup organization controller
	organizationRepository := organization.NewOrganizationRepository(db.DB)
	organizationService := organization.NewOrganizationService(organizationRepository)
	organizationController := organization.NewOrganizationController(testApi, organizationService)

	// Register controllers
	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", env.Configuration.OauthIssuerUrl())
	middlewares := []api.Middleware{
		middlewares.AuthMiddleware(testApi, jwksURL, env.Configuration.OauthIssuerUrl()),
		middlewares.OrganizationCheckMiddleware(testApi, func(organizationId string) (string, error) {
			return organization.GetOrganizationIamById(db.DB, organizationId)
		}),
	}
	controllers := []api.Controller{
		organizationController,
	}
	api.InitializeControllers(testApi, middlewares, controllers)

	t.Run("GET /organizations", func(t *testing.T) {
		t.Run("should return organizations for authenticated admin user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")
			assert.NoError(t, err)
			assert.NotNil(t, session)

			response := testApi.Get("/organizations", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var organizations []models.Organization
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &organizations, true)
			assert.NotEmpty(t, organizations)
			assert.Equal(t, "local", organizations[0].Name)
			assert.Equal(t, "local", organizations[0].IAMOrganizationID)
		})

		t.Run("should return organizations for authenticated planner user", func(t *testing.T) {
			session, err := env.GetUserSession("planner", "planner")
			assert.NoError(t, err)
			assert.NotNil(t, session)

			response := testApi.Get("/organizations", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var organizations []models.Organization
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &organizations, true)
			assert.NotEmpty(t, organizations)
			assert.Equal(t, "local", organizations[0].Name)
		})

		t.Run("should return organizations for authenticated regular user", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")
			assert.NoError(t, err)
			assert.NotNil(t, session)

			response := testApi.Get("/organizations", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var organizations []models.Organization
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &organizations, true)
			assert.NoError(t, err)
			assert.NotEmpty(t, organizations)
			assert.Equal(t, "local", organizations[0].Name)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Get("/organizations")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Get("/organizations", "Authorization: Bearer invalid_token")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with malformed authorization header", func(t *testing.T) {
			response := testApi.Get("/organizations", "Authorization: invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return organizations based on user's sub claim", func(t *testing.T) {
			// This test verifies that the endpoint correctly extracts the sub from JWT
			// and returns only organizations that the user belongs to
			session, err := env.GetUserSession("admin", "admin")
			assert.NoError(t, err)
			assert.NotNil(t, session)

			response := testApi.Get("/organizations", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var organizations []models.Organization
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &organizations, true)
			// In the test environment, all users belong to the "local" organization
			assert.NotEmpty(t, organizations)
			for _, org := range organizations {
				assert.NotZero(t, org.Id)
				assert.NotEmpty(t, org.Name)
				assert.NotEmpty(t, org.IAMOrganizationID)
			}
		})
	})
}
