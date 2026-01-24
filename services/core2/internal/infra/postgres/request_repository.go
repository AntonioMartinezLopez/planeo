package postgres

import (
	"context"
	"planeo/services/core2/internal/domain/request"

	"github.com/jackc/pgx/v5"
)

func (c *Client) GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool, selectedCategories []int) ([]request.Request, error) {
	var query string
	categoryFilter := ""
	if len(selectedCategories) > 0 {
		categoryFilter = " AND category_id = ANY(@categoryIds)"
	}

	if cursor == 0 {
		query = "SELECT * FROM requests WHERE organization_id = @organizationId AND id > @id" + categoryFilter + " ORDER BY id DESC FETCH FIRST @limit ROWS ONLY"
	} else {
		query = "SELECT * FROM requests WHERE organization_id = @organizationId AND id < @id" + categoryFilter + " ORDER BY id DESC FETCH FIRST @limit ROWS ONLY"
	}
	args := pgx.NamedArgs{"organizationId": organizationId, "id": cursor, "limit": limit, "categoryIds": selectedCategories}

	rows, err := c.db.Query(ctx, query, args)
	if err != nil {
		return nil, NewDatabaseError("error querying database", err)
	}

	requests, err := pgx.CollectRows(rows, pgx.RowToStructByName[request.Request])
	if err != nil {
		return nil, NewDatabaseError("error collecting requests", err)
	}

	return requests, nil
}

func (c *Client) GetRequest(ctx context.Context, organizationId int, requestId int) (request.Request, error) {
	query := `
		SELECT * FROM requests
		WHERE organization_id = @organizationId AND id = @requestId`
	args := pgx.NamedArgs{"organizationId": organizationId, "requestId": requestId}

	rows, err := c.db.Query(ctx, query, args)
	if err != nil {
		return request.Request{}, NewDatabaseError("error querying database", err)
	}

	req, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[request.Request])
	if err != nil {
		return request.Request{}, NewDatabaseError("error collecting request", err)
	}

	return req, nil
}

func (c *Client) CreateRequest(ctx context.Context, request request.NewRequest) (int, error) {
	query := `
		INSERT INTO requests (text, name, subject, email, address, telephone, raw, closed, reference_id, organization_id, category_id)
		VALUES (@text, @name, @subject, @email, @address, @telephone, @raw, @closed, @referenceId, @organizationId, @categoryId)
		RETURNING id`

	args := pgx.NamedArgs{
		"text":           request.Text,
		"name":           request.Name,
		"subject":        request.Subject,
		"email":          request.Email,
		"address":        request.Address,
		"telephone":      request.Telephone,
		"raw":            request.Raw,
		"referenceId":    request.ReferenceId,
		"closed":         request.Closed,
		"organizationId": request.OrganizationId,
		"categoryId":     nil,
	}

	if request.CategoryId != 0 {
		args["categoryId"] = request.CategoryId
	}

	var id int
	err := c.db.QueryRow(ctx, query, args).Scan(&id)
	if err != nil {
		return 0, NewDatabaseError("error inserting into database", err)
	}

	return id, nil
}

func (c *Client) UpdateRequest(ctx context.Context, req request.UpdateRequest) error {
	query := `
		UPDATE requests
		SET text = @text, name = @name, email = @email, address = @address, telephone = @telephone, closed = @closed, category_id = @categoryId
		WHERE organization_id = @organizationId AND id = @requestId`

	args := pgx.NamedArgs{
		"text":           req.Text,
		"name":           req.Name,
		"email":          req.Email,
		"address":        req.Address,
		"telephone":      req.Telephone,
		"closed":         req.Closed,
		"categoryId":     req.CategoryId,
		"organizationId": req.OrganizationId,
		"requestId":      req.Id,
	}

	if req.CategoryId == 0 {
		args["categoryId"] = nil
	}

	result, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error updating request", err)
	}

	if result.RowsAffected() == 0 {
		return request.RequestNotFoundError
	}

	return nil
}

func (c *Client) DeleteRequest(ctx context.Context, organizationId int, requestId int) error {
	query := `
		DELETE FROM requests
		WHERE organization_id = @organizationId AND id = @requestId`

	args := pgx.NamedArgs{"organizationId": organizationId, "requestId": requestId}

	result, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error deleting request", err)
	}

	if result.RowsAffected() == 0 {
		return request.RequestNotFoundError
	}

	return nil
}
