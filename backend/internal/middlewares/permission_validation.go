package middlewares

import (
	"errors"
	"net/http"
	jsonHelper "planeo/api/pkg/json"
)

func PermissionValidator(permission string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// get permission from context
			accessClaims, assertionCorrect := r.Context().Value(AccessClaimsContextKey{}).(OauthAccessClaims)

			if !assertionCorrect {
				jsonHelper.HttpErrorResponse(w, http.StatusInternalServerError, errors.New("no permissions found"))
				return
			}

			hasScope := accessClaims.HasScope(permission)

			if !hasScope {
				jsonHelper.HttpErrorResponse(w, http.StatusUnauthorized, errors.New("not authorized"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
