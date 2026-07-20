package middlewares_test

import (
	"encoding/json"
	"planeo/libs/middlewares"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOauthAccessClaimsUnmarshal(t *testing.T) {
	raw := []byte(`{
		"sub": "abc-123",
		"name": "admin admin",
		"given_name": "admin",
		"family_name": "Admin",
		"preferred_username": "admin",
		"email": "admin@local.de",
		"groups": ["/local"]
	}`)

	var claims middlewares.OauthAccessClaims
	err := json.Unmarshal(raw, &claims)

	assert.NoError(t, err)
	assert.Equal(t, "admin", claims.GivenName)
	assert.Equal(t, "Admin", claims.FamilyName)
	assert.Equal(t, "admin", claims.PreferredUsername)
}
