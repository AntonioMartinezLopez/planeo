package categories

import (
	"context"
	"net/http"
	humaUtils "planeo/libs/huma_utils"
	"planeo/libs/middlewares"
	"planeo/services/core2/internal/domain/category"
	. "planeo/services/core2/internal/infra/rest/api"

	"github.com/danielgtaylor/huma/v2"
)

type CategoriesHandler struct {
	categoryService category.Service
}

func NewCategoriesHandler(categoryService category.Service) *CategoriesHandler {
	return &CategoriesHandler{
		categoryService: categoryService,
	}
}

func (c *CategoriesHandler) GetCategories(ctx context.Context, input *GetCategoriesInput) (*GetCategoriesOutput, error) {
	result, err := c.categoryService.GetCategories(ctx, input.OrganizationId)
	if err != nil {
		return nil, NewHTTPError(err)
	}

	resp := &GetCategoriesOutput{}
	resp.Body.Categories = result

	return resp, nil
}

func (c *CategoriesHandler) CreateCategory(ctx context.Context, input *CreateCategoryInput) (*CreateCategoryOutput, error) {
	category := category.NewCategory{
		Label:            input.Body.Label,
		Color:            input.Body.Color,
		LabelDescription: input.Body.LabelDescription,
		OrganizationId:   input.OrganizationId,
	}

	id, err := c.categoryService.CreateCategory(ctx, input.OrganizationId, category)
	if err != nil {
		return nil, NewHTTPError(err)
	}
	resp := &CreateCategoryOutput{}
	resp.Body.Id = id

	return resp, nil
}

func (c *CategoriesHandler) UpdateCategory(ctx context.Context, input *UpdateCategoryInput) (*struct{}, error) {
	category := category.UpdateCategory{
		Id:               input.CategoryId,
		Label:            input.Body.Label,
		Color:            input.Body.Color,
		LabelDescription: input.Body.LabelDescription,
		OrganizationId:   input.OrganizationId,
	}

	err := c.categoryService.UpdateCategory(ctx, input.OrganizationId, input.CategoryId, category)
	if err != nil {
		return nil, NewHTTPError(err)
	}

	return nil, nil
}

func (c *CategoriesHandler) DeleteCategory(ctx context.Context, input *DeleteCategoryInput) (*struct{}, error) {
	err := c.categoryService.DeleteCategory(ctx, input.OrganizationId, input.CategoryId)
	if err != nil {
		return nil, NewHTTPError(err)
	}

	return nil, nil
}

func (c *CategoriesHandler) RegisterRoutes(api huma.API, permissions middlewares.PermissionMiddlewareConfig) {
	huma.Register(api, humaUtils.WithAuth(huma.Operation{
		OperationID: "get-categories",
		Method:      http.MethodGet,
		Path:        "/organizations/{organizationId}/categories",
		Summary:     "Get Categories",
		Tags:        []string{"Categories"},
		Middlewares: huma.Middlewares{permissions.Apply("category", "read")},
	}), c.GetCategories)

	huma.Register(api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "create-category",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/organizations/{organizationId}/categories",
		Summary:       "Create Category",
		Tags:          []string{"Categories"},
		Middlewares:   huma.Middlewares{permissions.Apply("category", "create")},
	}), c.CreateCategory)

	huma.Register(api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "update-category",
		Method:        http.MethodPut,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/categories/{categoryId}",
		Summary:       "Update Category",
		Tags:          []string{"Categories"},
		Middlewares:   huma.Middlewares{permissions.Apply("category", "update")},
	}), c.UpdateCategory)

	huma.Register(api, humaUtils.WithAuth(huma.Operation{
		OperationID:   "delete-category",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/organizations/{organizationId}/categories/{categoryId}",
		Summary:       "Delete Category",
		Tags:          []string{"Categories"},
		Middlewares:   huma.Middlewares{permissions.Apply("category", "delete")},
	}), c.DeleteCategory)
}
