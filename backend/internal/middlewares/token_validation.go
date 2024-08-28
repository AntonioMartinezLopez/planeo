package middlewares

import (
	"errors"
	"net/http"
	"os"
	"planeo/api/pkg/jwks"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/goccy/go-json"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
)

func isAuthorizationRequired(ctx huma.Context) bool {
	// 1. check whether auth needs to be applied
	isAuthorizationRequired := false
	for _, opScheme := range ctx.Operation().Security {
		var ok bool
		if _, ok = opScheme["bearer"]; ok {
			isAuthorizationRequired = true
			break
		}
	}
	return isAuthorizationRequired
}

func verifyToken(ctx huma.Context, keySet jwk.Set) ([]byte, error) {
	token := strings.TrimPrefix(ctx.Header("Authorization"), "Bearer ")
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

func verifyAccess(accessClaims *OauthAccessClaims, organization string, issuer string) bool {
	isCorrectAuthorizedParty := accessClaims.IsWithinOrganisation(organization)
	isIssuerCorrect := accessClaims.HasIssuer(issuer)

	if !isIssuerCorrect || !isCorrectAuthorizedParty {

		return false
	}

	return true
}

func AuthMiddleware(api huma.API, jwksURL string) func(ctx huma.Context, next func(huma.Context)) {
	keySet := jwks.NewJWKSet(jwksURL)
	issuer := os.Getenv("OAUTH_ISSUER")

	return func(ctx huma.Context, next func(huma.Context)) {
		authorizationRequired := isAuthorizationRequired(ctx)

		if !authorizationRequired {
			next(ctx)
			return
		}

		verifiedAccessToken, err := verifyToken(ctx, keySet)

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

		organization := strings.Split(ctx.URL().Path, "/")[2]
		validAccess := verifyAccess(accessClaims, organization, issuer)

		if !validAccess {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		// 5. add information to context
		ctx = huma.WithValue(ctx, AccessClaimsContextKey{}, *accessClaims)
		ctx = huma.WithValue(ctx, AccessTokenContextKey{}, string(verifiedAccessToken))

		next(ctx)
	}
}
