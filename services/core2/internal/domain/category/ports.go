package category

import (
	"context"
)

type CategoryRepositoryInterface interface {
	GetCategories(ctx context.Context, organizationId int) ([]Category, error)
	CreateCategory(ctx context.Context, organizationId int, category NewCategory) (int, error)
	UpdateCategory(ctx context.Context, organizationId int, categoryId int, category UpdateCategory) error
	DeleteCategory(ctx context.Context, organizationId int, categoryId int) error
}
