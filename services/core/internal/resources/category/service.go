package category

import (
	"context"
	"planeo/services/core/internal/resources/category/dto"
	"planeo/services/core/internal/resources/category/models"
)

type CategoryRepositoryInterface interface {
	GetCategories(ctx context.Context, organizationId int) ([]models.Category, error)
	CreateCategory(ctx context.Context, organizationId int, category dto.CreateCategoryInputBody) error
	UpdateCategory(ctx context.Context, organizationId int, categoryId int, category dto.UpdateCategoryInputBody) error
	DeleteCategory(ctx context.Context, organizationId int, categoryId int) error
}

type CategoryService struct {
	categoryRepository CategoryRepositoryInterface
}

func NewCategoryService(categoryRepository CategoryRepositoryInterface) *CategoryService {
	return &CategoryService{
		categoryRepository: categoryRepository,
	}
}

func (s *CategoryService) GetCategories(ctx context.Context, organizationId int) ([]models.Category, error) {
	return s.categoryRepository.GetCategories(ctx, organizationId)
}

func (s *CategoryService) CreateCategory(ctx context.Context, organizationId int, category dto.CreateCategoryInputBody) error {
	return s.categoryRepository.CreateCategory(ctx, organizationId, category)
}

func (s *CategoryService) UpdateCategory(ctx context.Context, organizationId int, categoryId int, category dto.UpdateCategoryInputBody) error {
	return s.categoryRepository.UpdateCategory(ctx, organizationId, categoryId, category)
}

func (s *CategoryService) DeleteCategory(ctx context.Context, organizationId int, categoryId int) error {
	return s.categoryRepository.DeleteCategory(ctx, organizationId, categoryId)
}
