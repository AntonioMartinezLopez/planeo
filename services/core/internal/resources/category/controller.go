package category

import (
	"context"
	"net/http"
	"planeo/libs/huma_utils"
	humaUtils "planeo/libs/huma_utils"
	"planeo/libs/middlewares"
	"planeo/services/core/config"
	"planeo/services/core/internal/resources/category/dto"
	"planeo/services/core/internal/resources/category/models"

	"github.com/danielgtaylor/huma/v2"
)

type CategoryController struct {
	api             huma.API
	categoryService *CategoryService
	config          *config.ApplicationConfiguration
}

func NewCategoryController(api huma.API, config *config.ApplicationConfiguration, categoryService *CategoryService) *CategoryController {
	return &CategoryController{
		api:             api,
		categoryService: categoryService,
		config:          config,
	}
}

func (c *CategoryController) InitializeRoutes() {
	permissions := middlewares.NewPermissionMiddlewareConfig(c.api, c.config.OauthIssuerUrl(), c.config.KcOauthClientID)
	huma.Register(c.api, humaUtils.WithAuth(huma.Operation{
		OperationID: "get-categories",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/categories",
		Summary:     "Get Categories",
		Tags:        []string{"Categories"},
		Middlewares: huma.Middlewares{permissions.Apply("category", "read")},
	}), func(ctx context.Context, input *dto.GetCategoriesInput) (*dto.GetCategoriesOutput, error) {
		result, err := c.categoryService.GetCategories(ctx, input.OrganizationId)
		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}
		resp := &dto.GetCategoriesOutput{}
		resp.Body.Categories = result
		return resp, nil
	})

	huma.Register(c.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "create-category",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/organizations/{organizationId}/categories",
		Summary:       "Create Category",
		Tags:          []string{"Categories"},
		Middlewares:   huma.Middlewares{permissions.Apply("category", "create")},
	}), func(ctx context.Context, input *dto.CreateCategoryInput) (*dto.CreateCategoryOutput, error) {
		category := models.NewCategory{
			Label:            input.Body.Label,
			Color:            input.Body.Color,
			LabelDescription: input.Body.LabelDescription,
			OrganizationId:   input.OrganizationId,
		}
		id, err := c.categoryService.CreateCategory(ctx, input.OrganizationId, category)
		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}
		resp := &dto.CreateCategoryOutput{}
		resp.Body.Id = id
		return resp, nil
	})

	huma.Register(c.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "update-category",
		Method:        http.MethodPut,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/categories/{categoryId}",
		Summary:       "Update Category",
		Tags:          []string{"Categories"},
		Middlewares:   huma.Middlewares{permissions.Apply("category", "update")},
	}), func(ctx context.Context, input *dto.UpdateCategoryInput) (*struct{}, error) {
		category := models.UpdateCategory{
			Id:               input.CategoryId,
			Label:            input.Body.Label,
			Color:            input.Body.Color,
			LabelDescription: input.Body.LabelDescription,
			OrganizationId:   input.OrganizationId,
		}
		err := c.categoryService.UpdateCategory(ctx, input.OrganizationId, input.CategoryId, category)
		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}
		return nil, nil
	})

	huma.Register(c.api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "delete-category",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/categories/{categoryId}",
		Summary:       "Delete Category",
		Tags:          []string{"Categories"},
		Middlewares:   huma.Middlewares{permissions.Apply("category", "delete")},
	}), func(ctx context.Context, input *dto.DeleteCategoryInput) (*struct{}, error) {
		err := c.categoryService.DeleteCategory(ctx, input.OrganizationId, input.CategoryId)
		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}
		return nil, nil
	})
}
