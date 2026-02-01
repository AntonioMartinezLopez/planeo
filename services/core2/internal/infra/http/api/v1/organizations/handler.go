package organizations

import (
	"context"
	"planeo/libs/middlewares"
	"planeo/services/core2/internal/domain/organization"
	"planeo/services/core2/internal/infra/http/server"

	"github.com/danielgtaylor/huma/v2"
)

type OrganizationHandler struct {
	organizationService organization.Service
}

func NewOrganizationHandler(api huma.API, organizationService organization.Service) *OrganizationHandler {
	return &OrganizationHandler{
		organizationService: organizationService,
	}
}

func (o *OrganizationHandler) GetOrganizations(ctx context.Context, input *GetOrganizationsInput) (*GetOrganizationsOutput, error) {
	// Get the sub claim from the JWT token
	claims, ok := ctx.Value(middlewares.AccessClaimsContextKey{}).(*middlewares.OauthAccessClaims)
	if !ok {
		return nil, huma.Error401Unauthorized("Unable to extract user claims")
	}

	userSub := claims.Sub
	if userSub == "" {
		return nil, huma.Error401Unauthorized("Missing sub claim in token")
	}

	foundOrganizations, err := o.organizationService.GetOrganizationsByUserSub(ctx, userSub)
	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	return &GetOrganizationsOutput{
		Body: foundOrganizations,
	}, nil
}
