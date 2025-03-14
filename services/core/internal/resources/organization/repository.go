package organization

import (
	"context"
	appError "planeo/libs/errors"
	"planeo/libs/logger"
	"planeo/services/core/internal/resources/organization/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrganizationRepository struct {
	db *pgxpool.Pool
}

func NewRequestRepository(database *pgxpool.Pool) *OrganizationRepository {
	return &OrganizationRepository{
		db: database,
	}
}

func (repo *OrganizationRepository) GetOrganizationIamById(ctx context.Context, id int) (models.Organization, error) {
	query := "SELECT * FROM organizations WHERE id = @id"
	args := pgx.NamedArgs{"id": id}

	row, err := repo.db.Query(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "GetOrganizationIamById").Msg("Error querying database")
		return models.Organization{}, appError.New(appError.InternalError, "Something went wrong", err)
	}

	return pgx.CollectOneRow(row, pgx.RowToStructByName[models.Organization])
}

func GetOrganizationIamById(db *pgxpool.Pool, id string) (string, error) {
	query := "SELECT * FROM organizations WHERE id = @id"
	args := pgx.NamedArgs{"id": id}

	row, err := db.Query(context.Background(), query, args)

	if err != nil {
		return "", err
	}

	organization, err := pgx.CollectOneRow(row, pgx.RowToStructByName[models.Organization])

	if err != nil {
		logger := logger.FromContext(context.Background())
		logger.Error().Err(err).Str("operation", "GetOrganizationIamById").Msg("Error querying database")
		return "", appError.New(appError.InternalError, "Something went wrong", err)
	}
	return organization.IAMOrganizationID, nil
}
