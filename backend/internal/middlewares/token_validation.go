package middlewares

import (
	"context"
	"errors"
	"net/http"
	"os"
	jsonHelper "planeo/api/pkg/json"
	"strings"
	"time"

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
	Permissions []string
}

func (c OauthAccessClaims) HasScope(expectedScope string) bool {
	for i := range c.Permissions {
		if c.Permissions[i] == expectedScope {
			return true
		}
	}
	return false
}

type AccessClaimsContextKey struct{}
type AccessTokenContextKey struct{}

func JwtValidator(next http.Handler) http.Handler {
	jwksURL := os.Getenv("JWKS_URL")
	keySet := newJWKSet(jwksURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		reqToken := r.Header.Get("Authorization")

		if len(reqToken) == 0 {
			jsonHelper.HttpErrorResponse(w, http.StatusUnauthorized, errors.New("missing access token"))
			return
		}

		splitToken := strings.Split(reqToken, "Bearer")
		if len(splitToken) != 2 {
			jsonHelper.HttpErrorResponse(w, http.StatusUnauthorized, errors.New("wrong authenticaton header format"))
			return
		}

		reqToken = strings.TrimSpace(splitToken[1])

		verifiedAccessToken, err := jws.Verify(
			[]byte(reqToken),
			jws.WithKeySet(keySet, jws.WithInferAlgorithmFromKey(true)),
		)

		if err != nil {
			jsonHelper.HttpErrorResponse(w, http.StatusUnauthorized, err)
			return
		}

		accessClaims := &OauthAccessClaims{}
		parseError := json.Unmarshal(verifiedAccessToken, accessClaims)

		if parseError != nil {
			jsonHelper.HttpErrorResponse(w, http.StatusInternalServerError, parseError)
			return
		}

		ctx := context.WithValue(r.Context(), AccessClaimsContextKey{}, *accessClaims)
		ctx = context.WithValue(ctx, AccessTokenContextKey{}, reqToken)

		// pass to next handler with extended context body
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
