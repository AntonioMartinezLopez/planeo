package test

import (
	"context"
	"fmt"
	"planeo/api/internal/resources/request"
	"planeo/api/internal/setup"
	"planeo/api/internal/test/utils"
	"planeo/api/pkg/db"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
)

func TestRequestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := utils.NewIntegrationTestEnvironment(t)
	db := db.InitializeDatabaseConnection(context.Background(), env.Configuration.DatabaseConfig())
	_, api := humatest.New(t)

	// setup request controller
	requestRepository := request.NewRequestRepository(db.DB)
	requestService := request.NewRequestService(requestRepository)
	requestController := request.NewRequestController(api, env.Configuration, requestService)

	// Register controllers
	setup.RegisterControllers(env.Configuration, api, db, []setup.Controller{requestController})

	t.Run("GET /requests ", func(t *testing.T) {

		t.Run("Test authorization for request access", func(t *testing.T) {
			testCases := []struct {
				name     string
				username string
				password string
			}{
				{"should return 200 when user is admin", "admin", "admin"},
				{"should return 200 when user is planner", "planner", "planner"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					session, err := env.GetUserSession(tc.username, tc.password)

					if err != nil {
						t.Error(err)
					}

					response := api.Get("/organizations/1/requests?pageSize=10", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))

					assert.Equal(t, 200, response.Code)
				})
			}
		})

	})
}
