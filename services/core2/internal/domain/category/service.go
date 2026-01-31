package category

import (
	"context"
)

type Service struct {
	categoryRepository CategoryRepository
}

func NewService(categoryRepository CategoryRepository) *Service {
	return &Service{
		categoryRepository: categoryRepository,
	}
}

func (s *Service) GetCategories(ctx context.Context, organizationId int) ([]Category, error) {
	return s.categoryRepository.GetCategories(ctx, organizationId)
}

func (s *Service) CreateCategory(ctx context.Context, organizationId int, category NewCategory) (int, error) {
	return s.categoryRepository.CreateCategory(ctx, organizationId, category)
}

func (s *Service) UpdateCategory(ctx context.Context, organizationId int, categoryId int, category UpdateCategory) error {
	return s.categoryRepository.UpdateCategory(ctx, organizationId, categoryId, category)
}

func (s *Service) DeleteCategory(ctx context.Context, organizationId int, categoryId int) error {
	return s.categoryRepository.DeleteCategory(ctx, organizationId, categoryId)
}
