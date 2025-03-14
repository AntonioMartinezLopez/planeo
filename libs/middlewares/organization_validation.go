package middlewares

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func OrganizationCheckMiddleware(api huma.API, resolveOrganization func(organizationId string) (string, error)) func(ctx huma.Context, next func(huma.Context)) {

	return func(ctx huma.Context, next func(huma.Context)) {
		accessClaims, assertionCorrect := ctx.Context().Value(AccessClaimsContextKey{}).(*OauthAccessClaims)

		if !assertionCorrect {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		organizationId := ctx.Param("organizationId")
		organization, err := resolveOrganization(organizationId)

		if err != nil {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		validAccess := accessClaims.IsWithinOrganisation(organization)

		if !validAccess {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		next(ctx)
	}
}
