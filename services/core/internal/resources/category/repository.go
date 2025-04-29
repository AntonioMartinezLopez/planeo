package category

import (
	"context"
	appError "planeo/libs/errors"
	"planeo/libs/logger"
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
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "GetCategories").Msg("Error querying database")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Category])
}

func (repo *CategoryRepository) CreateCategory(ctx context.Context, organizationId int, category models.NewCategory) (int, error) {
	query := `
        INSERT INTO categories (label, color, label_description, organization_id)
        VALUES (@label, @color, @labelDescription, @organizationId)
		RETURNING id`

	args := pgx.NamedArgs{
		"label":            category.Label,
		"color":            category.Color,
		"labelDescription": category.LabelDescription,
		"organizationId":   organizationId,
	}

	var id int

	err := repo.db.QueryRow(ctx, query, args).Scan(&id)
	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "CreateCategory").Msg("Error querying database")
		return 0, appError.New(appError.InternalError, "Something went wrong", err)
	}

	return id, nil
}

func (repo *CategoryRepository) UpdateCategory(ctx context.Context, organizationId int, categoryId int, category models.UpdateCategory) error {
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
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "UpdateCategory").Msg("Error querying database")
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
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "DeleteCategory").Msg("Error querying database")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	if result.RowsAffected() == 0 {
		return appError.New(appError.EntityNotFound, "Category not found", nil)
	}

	return nil
}
