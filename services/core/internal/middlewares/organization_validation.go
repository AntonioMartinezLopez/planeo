package middlewares

import (
	"context"
	"net/http"
	"planeo/libs/db"
	"planeo/libs/logger"
	cfg "planeo/services/core/config"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

func verifyAccess(accessClaims *OauthAccessClaims, organization string) bool {
	return accessClaims.IsWithinOrganisation(organization)
}

func resolveOrganization(organizationId string, database *db.DBConnection) (string, error) {
	query := "SELECT iam_organization_id FROM organizations WHERE id = @organizationId LIMIT 1"
	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := database.DB.Query(context.Background(), query, args)

	if err != nil {
		return "", err
	}

	type Organization struct {
		IamOrganizationID string `json:"iam_organization_id" db:"iam_organization_id"`
	}
	organization := Organization{}

	if rows.Next() {
		err = rows.Scan(&organization.IamOrganizationID)
		if err != nil {
			logger.Error("Error scanning row: %s", err.Error())
			return "", err
		}
	}
	rows.Close()

	if err != nil {
		return "", err
	}
	return organization.IamOrganizationID, nil

}

func OrganizationCheckMiddleware(api huma.API, config *cfg.ApplicationConfiguration, database *db.DBConnection) func(ctx huma.Context, next func(huma.Context)) {

	return func(ctx huma.Context, next func(huma.Context)) {
		accessClaims, assertionCorrect := ctx.Context().Value(AccessClaimsContextKey{}).(*OauthAccessClaims)

		if !assertionCorrect {
			logger.Error("Assertion not correct")
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		organizationId := ctx.Param("organizationId")
		organization, err := resolveOrganization(organizationId, database)

		if err != nil {
			logger.Error("Organization not found")
			huma.WriteErr(api, ctx, http.StatusNotFound, err.Error())
			return
		}

		validAccess := verifyAccess(accessClaims, organization)

		if !validAccess {
			huma.WriteErr(api, ctx, http.StatusForbidden, "Forbidden")
			return
		}

		next(ctx)
	}
}
