package request

import (
	"context"
	appError "planeo/libs/errors"
	"planeo/libs/logger"
	"planeo/services/core/internal/resources/request/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RequestRepository struct {
	db *pgxpool.Pool
}

func NewRequestRepository(database *pgxpool.Pool) *RequestRepository {
	return &RequestRepository{
		db: database,
	}
}

func (repo *RequestRepository) GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool) ([]models.Request, error) {

	var query string
	if cursor == 0 {
		query = "SELECT * FROM requests WHERE organization_id = @organizationId AND id > @id ORDER BY id DESC FETCH FIRST @limit ROWS ONLY"
	} else {
		query = "SELECT * FROM requests WHERE organization_id = @organizationId AND id < @id ORDER BY id DESC FETCH FIRST @limit ROWS ONLY"
	}
	args := pgx.NamedArgs{"organizationId": organizationId, "id": cursor, "limit": limit}

	rows, err := repo.db.Query(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "GetRequests").Msg("Error querying database")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Request])
}

func (repo *RequestRepository) CreateRequest(ctx context.Context, request models.NewRequest) error {

	query := `
		INSERT INTO requests (text, name, email, address, telephone, closed, organization_id, category_id)
		VALUES (@text, @name, @email, @address, @telephone, @closed, @organizationId, @categoryId)`

	args := pgx.NamedArgs{
		"text":           request.Text,
		"name":           request.Name,
		"email":          request.Email,
		"address":        request.Address,
		"telephone":      request.Telephone,
		"closed":         request.Closed,
		"organizationId": request.OrganizationId,
		"categoryId":     nil,
	}

	if request.CategoryId != 0 {
		args["categoryId"] = request.CategoryId
	}

	_, err := repo.db.Exec(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "CreateRequest").Msg("Error inserting into database")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (repo *RequestRepository) UpdateRequest(ctx context.Context, request models.UpdateRequest) error {

	query := `
		UPDATE requests
		SET text = @text, name = @name, email = @email, address = @address, telephone = @telephone, closed = @closed, category_id = @categoryId
		WHERE organization_id = @organizationId AND id = @requestId`

	args := pgx.NamedArgs{
		"text":           request.Text,
		"name":           request.Name,
		"email":          request.Email,
		"address":        request.Address,
		"telephone":      request.Telephone,
		"closed":         request.Closed,
		"categoryId":     request.CategoryId,
		"organizationId": request.OrganizationId,
		"requestId":      request.Id,
	}

	result, err := repo.db.Exec(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "UpdateRequest").Msg("Error updating request")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	if result.RowsAffected() == 0 {
		return appError.New(appError.EntityNotFound, "Request not found", nil)
	}

	return nil
}

func (repo *RequestRepository) DeleteRequest(ctx context.Context, organizationId int, requestId int) error {

	query := `
		DELETE FROM requests
		WHERE organization_id = @organizationId AND id = @requestId`

	args := pgx.NamedArgs{"organizationId": organizationId, "requestId": requestId}

	result, err := repo.db.Exec(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "DeleteRequest").Msg("Error deleting request")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	if result.RowsAffected() == 0 {
		return appError.New(appError.EntityNotFound, "Request not found", nil)
	}

	return nil
}
