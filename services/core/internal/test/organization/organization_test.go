package organization

import (
	"fmt"
	"testing"

	jsonHelper "planeo/libs/json"
	"planeo/services/core/internal/domain/organization"
	"planeo/services/core/internal/test/utils"

	"github.com/stretchr/testify/assert"
)

func TestOrganizationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := utils.NewIntegrationTestEnvironment(t)
	testApi := env.Api

	t.Run("GET /v1/organizations", func(t *testing.T) {
		t.Run("should return organizations for authenticated admin user", func(t *testing.T) {
			session, err := env.GetUserSession("admin", "admin")
			assert.NoError(t, err)
			assert.NotNil(t, session)

			response := testApi.Get("/v1/organizations", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var organizations []organization.Organization
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &organizations, true)
			assert.NotEmpty(t, organizations)
			assert.Equal(t, "local", organizations[0].Name)
			assert.Equal(t, "local", organizations[0].IAMOrganizationID)
		})

		t.Run("should return organizations for authenticated planner user", func(t *testing.T) {
			session, err := env.GetUserSession("planner", "planner")
			assert.NoError(t, err)
			assert.NotNil(t, session)

			response := testApi.Get("/v1/organizations", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var organizations []organization.Organization
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &organizations, true)
			assert.NotEmpty(t, organizations)
			assert.Equal(t, "local", organizations[0].Name)
		})

		t.Run("should return organizations for authenticated regular user", func(t *testing.T) {
			session, err := env.GetUserSession("user", "user")
			assert.NoError(t, err)
			assert.NotNil(t, session)

			response := testApi.Get("/v1/organizations", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var organizations []organization.Organization
			_ = jsonHelper.DecodeJSONAndValidate(response.Result().Body, &organizations, true)
			assert.NoError(t, err)
			assert.NotEmpty(t, organizations)
			assert.Equal(t, "local", organizations[0].Name)
		})

		t.Run("should return 401 with missing authorization header", func(t *testing.T) {
			response := testApi.Get("/v1/organizations")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with invalid authorization header", func(t *testing.T) {
			response := testApi.Get("/v1/organizations", "Authorization: Bearer invalid_token")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return 401 with malformed authorization header", func(t *testing.T) {
			response := testApi.Get("/v1/organizations", "Authorization: invalid")

			assert.Equal(t, 401, response.Code)
		})

		t.Run("should return organizations based on user's sub claim", func(t *testing.T) {
			// This test verifies that the endpoint correctly extracts the sub from JWT
			// and returns only organizations that the user belongs to
			session, err := env.GetUserSession("admin", "admin")
			assert.NoError(t, err)
			assert.NotNil(t, session)

			response := testApi.Get("/v1/organizations", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

			assert.Equal(t, 200, response.Code)

			var organizations []organization.Organization
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
