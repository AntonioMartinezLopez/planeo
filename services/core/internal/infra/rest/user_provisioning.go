package rest

import (
	"strings"

	"planeo/libs/logger"
	"planeo/libs/middlewares"
	"planeo/services/core/internal/domain/organization"
	"planeo/services/core/internal/domain/user"

	"github.com/danielgtaylor/huma/v2"
)

// ProvisionUserMiddleware ensures the authenticated caller has a matching
// Postgres users row before the request reaches its handler. Keycloak is
// the source of truth for identity; this mirrors that identity into
// Postgres lazily, on whichever authenticated request happens to arrive
// first for a given sub, instead of requiring the two systems' ids to be
// pre-synced out of band (see
// docs/superpowers/specs/2026-07-20-jit-user-provisioning-design.md).
//
// This is best-effort: any failure here is logged and the request proceeds
// regardless. Most routes never touch the users table, and a transient
// failure self-heals on the next authenticated request since provisioning
// is attempted on every one.
func ProvisionUserMiddleware(userService user.Service, organizationService organization.Service) Middleware {
	return func(ctx huma.Context, next func(huma.Context)) {
		accessClaims, assertionCorrect := ctx.Context().Value(middlewares.AccessClaimsContextKey{}).(*middlewares.OauthAccessClaims)

		if !assertionCorrect || len(accessClaims.Groups) == 0 {
			next(ctx)
			return
		}

		log := logger.FromContext(ctx.Context())
		iamOrganizationId := strings.TrimPrefix(accessClaims.Groups[0], "/")

		org, err := organizationService.GetOrganizationByIAMId(ctx.Context(), iamOrganizationId)
		if err != nil {
			log.Error().Err(err).Str("iamOrganizationId", iamOrganizationId).Msg("failed to resolve organization for user provisioning")
			next(ctx)
			return
		}

		err = userService.EnsureProvisioned(
			ctx.Context(),
			org.Id,
			accessClaims.Sub,
			accessClaims.PreferredUsername,
			accessClaims.GivenName,
			accessClaims.FamilyName,
			accessClaims.Email,
		)
		if err != nil {
			log.Error().Err(err).Str("sub", accessClaims.Sub).Msg("failed to provision user")
		}

		next(ctx)
	}
}
