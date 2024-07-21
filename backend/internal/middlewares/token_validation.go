package middlewares

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/goccy/go-json"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
)

func newJWKSet(jwkUrl string) jwk.Set {
	jwkCache := jwk.NewCache(context.Background())

	// register a minimum refresh interval for this URL.
	// when not specified, defaults to Cache-Control and similar resp headers
	err := jwkCache.Register(jwkUrl, jwk.WithMinRefreshInterval(10*time.Minute))
	if err != nil {
		panic("failed to register jwk location")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// fetch once on application startup
	_, err = jwkCache.Refresh(ctx, jwkUrl)
	if err != nil {
		panic("failed to fetch on startup")
	}
	// create the cached key set
	return jwk.NewCachedSet(jwkCache, jwkUrl)
}

type OauthAccessClaims struct {
	Permissions []string `json:"permissions"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	UserId      string   `json:"userid"`
	Issuer      string   `json:"iss"`
	Audiences   []string `json:"aud"`
}

func (c OauthAccessClaims) HasScope(expectedScope string) bool {
	for i := range c.Permissions {
		if c.Permissions[i] == expectedScope {
			return true
		}
	}
	return false
}

func (c OauthAccessClaims) HasAudience(expectedAudience string) bool {
	for i := range c.Audiences {
		if c.Audiences[i] == expectedAudience {
			return true
		}
	}
	return false
}

func (c OauthAccessClaims) HasIssuer(expectedIssuer string) bool {
	return c.Issuer == expectedIssuer
}

type AccessClaimsContextKey struct{}
type AccessTokenContextKey struct{}

// NewAuthMiddleware creates a middleware that will authorize requests based on
// the required scopes for the operation.
func AuthMiddleware(api huma.API, jwksURL string) func(ctx huma.Context, next func(huma.Context)) {
	keySet := newJWKSet(jwksURL)

	return func(ctx huma.Context, next func(huma.Context)) {

		// 1. check whether auth needs to be applied
		isAuthorizationRequired := false
		for _, opScheme := range ctx.Operation().Security {
			var ok bool
			if _, ok = opScheme["myAuth"]; ok {
				isAuthorizationRequired = true
				break
			}
		}

		if !isAuthorizationRequired {
			next(ctx)
			return
		}

		// 2. extract token and verfiy
		token := strings.TrimPrefix(ctx.Header("Authorization"), "Bearer ")
		if len(token) == 0 {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			return
		}

		verifiedAccessToken, err := jws.Verify(
			[]byte(token),
			jws.WithKeySet(keySet, jws.WithInferAlgorithmFromKey(true)),
		)

		if err != nil {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			return
		}

		accessClaims := &OauthAccessClaims{}
		parseError := json.Unmarshal(verifiedAccessToken, accessClaims)

		if parseError != nil {
			huma.WriteErr(api, ctx, http.StatusInternalServerError, parseError.Error())
			return
		}

		// 3. verfiy audience and Issuer
		isAudienceCorrect := accessClaims.HasAudience("https://api.planeo.de")
		isIssuerCorrect := accessClaims.HasIssuer(os.Getenv("OAUTH_ISSUER"))

		if !isAudienceCorrect || isIssuerCorrect {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		// 4. add information to context
		ctx = huma.WithValue(ctx, AccessClaimsContextKey{}, *accessClaims)
		ctx = huma.WithValue(ctx, AccessTokenContextKey{}, token)

		next(ctx)
	}
}
