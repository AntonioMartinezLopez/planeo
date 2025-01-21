package user

import (
	"fmt"
	"planeo/api/internal/utils/integration_tests"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserIntegration(t *testing.T) {

	if testing.Short() {
		t.Skip()
	}

	// Start integration environment
	env := integration_tests.NewIntegrationTestEnvironment(t)

	t.Run("CreateUser", func(t *testing.T) {

		fmt.Println(env.Configuration)
		session, err := env.GetUserSession("admin", "admin")

		if err != nil {
			t.Error(err)
		}

		assert.NotNil(t, session)
	})
}
