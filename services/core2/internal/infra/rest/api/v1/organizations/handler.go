package organizations

import (
	"context"
	"net/http"
	humaUtils "planeo/libs/huma_utils"
	"planeo/libs/middlewares"
	"planeo/services/core2/internal/domain/organization"
	. "planeo/services/core2/internal/infra/rest/api"

	"github.com/danielgtaylor/huma/v2"
)

type OrganizationHandler struct {
	organizationService organization.Service
}

func NewOrganizationHandler(organizationService organization.Service) *OrganizationHandler {
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
		return nil, NewHTTPError(err)
	}

	return &GetOrganizationsOutput{
		Body: foundOrganizations,
	}, nil
}

func (o *OrganizationHandler) RegisterRoutes(api huma.API, permissions middlewares.PermissionMiddlewareConfig) {
	huma.Register(api, humaUtils.WithAuth(huma.Operation{
		OperationID: "get-organizations",
		Method:      http.MethodGet,
		Path:        "/organizations",
		Summary:     "Get Organizations for Current User",
		Description: "Returns all organizations that the authenticated user belongs to, based on the sub claim from the JWT token.",
		Tags:        []string{"Organizations"},
	}), o.GetOrganizations)
}
