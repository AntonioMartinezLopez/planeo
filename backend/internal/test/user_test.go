package test

import (
	"context"
	"fmt"

	"planeo/api/internal/setup"
	"planeo/api/internal/test/utils"
	"planeo/api/pkg/db"
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
	db := db.InitializeDatabaseConnection(context.Background(), env.Configuration.DatabaseConfig())
	_, api := humatest.New(t)
	setup.SetupRoutes(api, env.Configuration, db)

	t.Run("CreateUser", func(t *testing.T) {

		fmt.Println(env.Configuration)
		session, err := env.GetUserSession("admin", "admin")

		if err != nil {
			t.Error(err)
		}

		assert.NotNil(t, session)
		response := api.Get("/local/admin/users", fmt.Sprintf("Authorization: Bearer %s", session.AccessToken))
		assert.Equal(t, 200, response.Code)
	})
}
