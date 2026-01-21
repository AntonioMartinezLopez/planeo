package postgres

import (
	"context"
	"planeo/services/core2/internal/domain/category"

	"github.com/jackc/pgx/v5"
)

func (c *Client) GetCategories(ctx context.Context, organizationId int) ([]category.Category, error) {
	query := `SELECT * FROM categories WHERE organization_id = @organizationId ORDER by id`

	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := c.db.Query(ctx, query, args)

	if err != nil {
		return nil, NewDatabaseError("failed to query categories", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[category.Category])
}

func (c *Client) CreateCategory(ctx context.Context, organizationId int, newCategory category.NewCategory) (int, error) {
	query := `
        INSERT INTO categories (label, color, label_description, organization_id)
        VALUES (@label, @color, @labelDescription, @organizationId)
		RETURNING id`

	args := pgx.NamedArgs{
		"label":            newCategory.Label,
		"color":            newCategory.Color,
		"labelDescription": newCategory.LabelDescription,
		"organizationId":   organizationId,
	}

	var id int

	err := c.db.QueryRow(ctx, query, args).Scan(&id)
	if err != nil {
		return 0, NewDatabaseError("failed to create category", err)
	}

	return id, nil
}

func (c *Client) UpdateCategory(ctx context.Context, organizationId int, categoryId int, updateCategory category.UpdateCategory) error {
	query := `
        UPDATE categories
        SET label = @label, color = @color, label_description = @labelDescription
        WHERE id = @categoryId AND organization_id = @organizationId`

	args := pgx.NamedArgs{
		"label":            updateCategory.Label,
		"color":            updateCategory.Color,
		"labelDescription": updateCategory.LabelDescription,
		"categoryId":       categoryId,
		"organizationId":   organizationId,
	}

	result, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("failed to update category", err)
	}

	if result.RowsAffected() == 0 {
		return &category.CategoryNotFoundError
	}

	return nil
}

func (c *Client) DeleteCategory(ctx context.Context, organizationId int, categoryId int) error {
	query := `
        DELETE FROM categories
        WHERE id = @categoryId AND organization_id = @organizationId`

	args := pgx.NamedArgs{"categoryId": categoryId, "organizationId": organizationId}

	result, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("failed to delete category", err)
	}

	if result.RowsAffected() == 0 {
		return &category.CategoryNotFoundError
	}

	return nil
}
