package category

import (
	"context"
	"net/http"
	"planeo/api/config"
	"planeo/api/internal/middlewares"
	"planeo/api/internal/resources/category/dto"
	"planeo/api/internal/setup/operations"
	"planeo/api/internal/utils/huma_utils"

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
	huma.Register(c.api, operations.WithAuth(huma.Operation{
		OperationID: "get-categories",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/categories",
		Summary:     "Get Categories",
		Tags:        []string{"Categories"},
		Middlewares: huma.Middlewares{middlewares.PermissionMiddleware(c.api, c.config, "category", "read")},
	}), func(ctx context.Context, input *dto.GetCategoriesInput) (*dto.GetCategoriesOutput, error) {
		result, err := c.categoryService.GetCategories(ctx, input.OrganizationId)
		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}
		resp := &dto.GetCategoriesOutput{}
		resp.Body.Categories = result
		return resp, nil
	})

	huma.Register(c.api, operations.WithAuth(huma.Operation{
		OperationID:   "create-category",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/organizations/{organizationId}/categories",
		Summary:       "Create Category",
		Tags:          []string{"Categories"},
		Middlewares:   huma.Middlewares{middlewares.PermissionMiddleware(c.api, c.config, "category", "create")},
	}), func(ctx context.Context, input *dto.CreateCategoryInput) (*struct{}, error) {
		err := c.categoryService.CreateCategory(ctx, input.OrganizationId, input.Body)
		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}
		return nil, nil
	})

	huma.Register(c.api, operations.WithAuth(huma.Operation{
		OperationID:   "update-category",
		Method:        http.MethodPut,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/categories/{categoryId}",
		Summary:       "Update Category",
		Tags:          []string{"Categories"},
		Middlewares:   huma.Middlewares{middlewares.PermissionMiddleware(c.api, c.config, "category", "update")},
	}), func(ctx context.Context, input *dto.UpdateCategoryInput) (*struct{}, error) {
		err := c.categoryService.UpdateCategory(ctx, input.OrganizationId, input.CategoryId, input.Body)
		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}
		return nil, nil
	})

	huma.Register(c.api, operations.WithAuth(huma.Operation{
		OperationID:   "delete-category",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/categories/{categoryId}",
		Summary:       "Delete Category",
		Tags:          []string{"Categories"},
		Middlewares:   huma.Middlewares{middlewares.PermissionMiddleware(c.api, c.config, "category", "delete")},
	}), func(ctx context.Context, input *dto.DeleteCategoryInput) (*struct{}, error) {
		err := c.categoryService.DeleteCategory(ctx, input.OrganizationId, input.CategoryId)
		if err != nil {
			return nil, huma_utils.NewHumaError(err)
		}
		return nil, nil
	})
}
