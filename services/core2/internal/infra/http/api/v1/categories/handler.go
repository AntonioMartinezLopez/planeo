package categories

import (
	"context"
	"planeo/services/core2/internal/domain/category"
	"planeo/services/core2/internal/infra/http/server"
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
		return nil, server.NewHTTPError(err)
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
		return nil, server.NewHTTPError(err)
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
		return nil, server.NewHTTPError(err)
	}

	return nil, nil
}

func (c *CategoriesHandler) DeleteCategory(ctx context.Context, input *DeleteCategoryInput) (*struct{}, error) {
	err := c.categoryService.DeleteCategory(ctx, input.OrganizationId, input.CategoryId)
	if err != nil {
		return nil, server.NewHTTPError(err)
	}

	return nil, nil
}
