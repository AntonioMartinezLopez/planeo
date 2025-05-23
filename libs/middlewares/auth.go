package middlewares

import (
	"errors"
	"net/http"
	"planeo/libs/jwks"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/goccy/go-json"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
)

func isAuthorizationRequired(ctx huma.Context) bool {
	for _, opScheme := range ctx.Operation().Security {
		if _, ok := opScheme["bearer"]; ok {
			return true
		}
	}
	return false
}

func verifyToken(token string, keySet jwk.Set) ([]byte, error) {
	if len(token) == 0 {
		return nil, errors.New("missing auth token")
	}

	verifiedAccessToken, err := jws.Verify(
		[]byte(token),
		jws.WithKeySet(keySet, jws.WithInferAlgorithmFromKey(true)),
	)

	if err != nil {
		return nil, errors.New("invalid auth token")
	}

	return verifiedAccessToken, nil
}

func parseToken(token []byte) (*OauthAccessClaims, error) {
	accessClaims := &OauthAccessClaims{}
	parseError := json.Unmarshal(token, accessClaims)

	if parseError != nil {
		return nil, parseError
	}

	return accessClaims, nil
}

func AuthMiddleware(api huma.API, jwksURL string, issuer string) func(ctx huma.Context, next func(huma.Context)) {
	keySet := jwks.NewJWKSet(jwksURL)

	return func(ctx huma.Context, next func(huma.Context)) {
		authorizationRequired := isAuthorizationRequired(ctx)

		if !authorizationRequired {
			next(ctx)
			return
		}

		token := strings.TrimPrefix(ctx.Header("Authorization"), "Bearer ")
		verifiedAccessToken, err := verifyToken(token, keySet)

		if err != nil {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, err.Error())
			return
		}

		accessClaims, err := parseToken(verifiedAccessToken)

		if err != nil {
			huma.WriteErr(api, ctx, http.StatusInternalServerError, err.Error())
			return
		}

		if accessClaims.IsExpired() {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Access token expired")
			return
		}

		ctx = huma.WithValue(ctx, AccessClaimsContextKey{}, accessClaims)
		ctx = huma.WithValue(ctx, AccessTokenContextKey{}, token)

		next(ctx)
	}
}
