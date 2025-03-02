package category

import (
	"context"
	appError "planeo/services/core/internal/errors"
	"planeo/services/core/internal/resources/category/dto"
	"planeo/services/core/internal/resources/category/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(database *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{
		db: database,
	}
}
func (repo *CategoryRepository) GetCategories(ctx context.Context, organizationId int) ([]models.Category, error) {
	query := `SELECT * FROM categories WHERE organization_id = @organizationId ORDER by id`

	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := repo.db.Query(ctx, query, args)
	if err != nil {
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Category])
}

func (repo *CategoryRepository) CreateCategory(ctx context.Context, organizationId int, category dto.CreateCategoryInputBody) error {
	query := `
        INSERT INTO categories (label, color, label_description, organization_id)
        VALUES (@label, @color, @labelDescription, @organizationId)`

	args := pgx.NamedArgs{
		"label":            category.Label,
		"color":            category.Color,
		"labelDescription": category.LabelDescription,
		"organizationId":   organizationId,
	}

	_, err := repo.db.Exec(ctx, query, args)
	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (repo *CategoryRepository) UpdateCategory(ctx context.Context, organizationId int, categoryId int, category dto.UpdateCategoryInputBody) error {
	query := `
        UPDATE categories
        SET label = @label, color = @color, label_description = @labelDescription
        WHERE id = @categoryId AND organization_id = @organizationId`

	args := pgx.NamedArgs{
		"label":            category.Label,
		"color":            category.Color,
		"labelDescription": category.LabelDescription,
		"categoryId":       categoryId,
		"organizationId":   organizationId,
	}

	result, err := repo.db.Exec(ctx, query, args)
	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	if result.RowsAffected() == 0 {
		return appError.New(appError.EntityNotFound, "Category not found", nil)
	}

	return nil
}

func (repo *CategoryRepository) DeleteCategory(ctx context.Context, organizationId int, categoryId int) error {
	query := `
        DELETE FROM categories
        WHERE id = @categoryId AND organization_id = @organizationId`

	args := pgx.NamedArgs{"categoryId": categoryId, "organizationId": organizationId}

	result, err := repo.db.Exec(ctx, query, args)
	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	if result.RowsAffected() == 0 {
		return appError.New(appError.EntityNotFound, "Category not found", nil)
	}

	return nil
}
