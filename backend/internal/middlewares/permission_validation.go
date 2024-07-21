package middlewares

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func PermissionMiddleware(api huma.API, permission string) func(ctx huma.Context, next func(huma.Context)) {

	return func(ctx huma.Context, next func(huma.Context)) {
		accessClaims, assertionCorrect := ctx.Context().Value(AccessClaimsContextKey{}).(OauthAccessClaims)

		if !assertionCorrect {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		hasScope := accessClaims.HasScope(permission)

		if !hasScope {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			return
		}

		next(ctx)
	}
}
