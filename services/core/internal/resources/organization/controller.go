package organization

import (
	"context"
	"net/http"
	humaUtils "planeo/libs/huma_utils"
	"planeo/libs/middlewares"
	"planeo/services/core/internal/resources/organization/dto"

	"github.com/danielgtaylor/huma/v2"
)

type OrganizationController struct {
	api                 huma.API
	organizationService *OrganizationService
}

func NewOrganizationController(api huma.API, organizationService *OrganizationService) *OrganizationController {
	return &OrganizationController{
		api:                 api,
		organizationService: organizationService,
	}
}

func (c *OrganizationController) InitializeRoutes() {
	huma.Register(c.api, humaUtils.WithAuth(huma.Operation{
		OperationID: "get-organizations",
		Method:      http.MethodGet,
		Path:        "/organizations",
		Summary:     "Get Organizations for Current User",
		Description: "Returns all organizations that the authenticated user belongs to, based on the sub claim from the JWT token.",
		Tags:        []string{"Organizations"},
	}), func(ctx context.Context, input *dto.GetOrganizationsInput) (*dto.GetOrganizationsOutput, error) {
		// Get the sub claim from the JWT token
		claims, ok := ctx.Value(middlewares.AccessClaimsContextKey{}).(*middlewares.OauthAccessClaims)
		if !ok {
			return nil, huma.Error401Unauthorized("Unable to extract user claims")
		}

		userSub := claims.Sub
		if userSub == "" {
			return nil, huma.Error401Unauthorized("Missing sub claim in token")
		}

		organizations, err := c.organizationService.GetOrganizationsByUserSub(ctx, userSub)
		if err != nil {
			return nil, humaUtils.NewHumaError(err)
		}

		return &dto.GetOrganizationsOutput{
			Body: organizations,
		}, nil
	})
}
