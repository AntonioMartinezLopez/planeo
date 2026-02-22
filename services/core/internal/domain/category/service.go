package category

import (
	"context"
)

type service struct {
	categoryRepository CategoryRepository
}

func NewService(categoryRepository CategoryRepository) Service {
	return &service{
		categoryRepository: categoryRepository,
	}
}

func (s *service) GetCategories(ctx context.Context, organizationId int) ([]Category, error) {
	return s.categoryRepository.GetCategories(ctx, organizationId)
}

func (s *service) CreateCategory(ctx context.Context, organizationId int, category NewCategory) (int, error) {
	return s.categoryRepository.CreateCategory(ctx, organizationId, category)
}

func (s *service) UpdateCategory(ctx context.Context, organizationId int, categoryId int, category UpdateCategory) error {
	return s.categoryRepository.UpdateCategory(ctx, organizationId, categoryId, category)
}

func (s *service) DeleteCategory(ctx context.Context, organizationId int, categoryId int) error {
	return s.categoryRepository.DeleteCategory(ctx, organizationId, categoryId)
}
