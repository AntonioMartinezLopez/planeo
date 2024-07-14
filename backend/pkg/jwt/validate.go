package jwt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	jsonHelper "planeo/api/pkg/json"
	"strings"
	"time"

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

func JwtValidator(next http.Handler) http.Handler {
	jwksURL := os.Getenv("JWKS_URL")
	keySet := newJWKSet(jwksURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// extract authorization header
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

		fmt.Println(reqToken)

		verifiedAccessToken, err := jws.Verify(
			[]byte(reqToken),
			jws.WithKeySet(keySet, jws.WithInferAlgorithmFromKey(true)),
		)

		if err != nil {
			jsonHelper.HttpErrorResponse(w, http.StatusUnauthorized, errors.New("wrong authenticaton header format"))
		}

		accessClaims := &OauthAccessClaims{}
		parseError := jsonHelper.DecodeJSON(bytes.NewReader(verifiedAccessToken), accessClaims)

		if parseError != nil {

		}

		// // extract user claims from header set by api gateway (did the authentication step)
		// userClaims := r.Header.Get("X-Auth-User-Claims")
		// if userClaims == "" {
		// 	jsonhelper.HttpErrorResponse(w, http.StatusUnauthorized, errors.New("Missing user claims"))
		// 	return
		// }

		// // decode user claims
		// decodedUserClaims := Claims{}
		// err := json.Unmarshal([]byte(userClaims), &decodedUserClaims)

		// if err != nil {
		// 	jsonHelper.HttpErrorResponse(w, http.StatusInternalServerError, err)
		// 	return
		// }

		// // extend context with user claim information
		// ctx := context.WithValue(r.Context(), "user-claims", decodedUserClaims)

		// pass to next handler with extended context body
		// next.ServeHTTP(w, r.WithContext(ctx))
		next.ServeHTTP(w, r)
	})
}
