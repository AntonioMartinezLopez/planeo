package middlewares

import (
	"net/http"
	"os"
	"planeo/api/pkg/jwk"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/goccy/go-json"
	"github.com/lestrrat-go/jwx/v2/jws"
)

func AuthMiddleware(api huma.API, jwksURL string) func(ctx huma.Context, next func(huma.Context)) {
	keySet := jwk.NewJWKSet(jwksURL)
	issuer := os.Getenv("OAUTH_ISSUER")

	return func(ctx huma.Context, next func(huma.Context)) {
		// 1. check whether auth needs to be applied
		isAuthorizationRequired := false
		for _, opScheme := range ctx.Operation().Security {
			var ok bool
			if _, ok = opScheme["bearer"]; ok {
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

		// 3. Check expiration
		if accessClaims.IsExpired() {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Access token expired")
			return
		}

		// 4. verfiy audience and Issuer
		organization := strings.Split(ctx.URL().Path, "/")[2]
		isCorrectAuthorizedParty := accessClaims.IsCorrectAzp(organization)
		isIssuerCorrect := accessClaims.HasIssuer(issuer)

		if !isIssuerCorrect || !isCorrectAuthorizedParty {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		// 5. add information to context
		ctx = huma.WithValue(ctx, AccessClaimsContextKey{}, *accessClaims)
		ctx = huma.WithValue(ctx, AccessTokenContextKey{}, token)

		next(ctx)
	}
}
