package postgres

import (
	"context"
	appError "planeo/libs/errors"
	"planeo/libs/logger"
	"planeo/services/core2/internal/domain/organization"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (c *Client) GetOrganizationIamById(ctx context.Context, id int) (organization.Organization, error) {
	query := "SELECT * FROM organizations WHERE id = @id"
	args := pgx.NamedArgs{"id": id}

	row, err := c.db.Query(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "GetOrganizationIamById").Msg("Error querying database")
		return organization.Organization{}, appError.New(appError.InternalError, "Something went wrong", err)
	}

	return pgx.CollectOneRow(row, pgx.RowToStructByName[organization.Organization])
}

func GetOrganizationIamById(db *pgxpool.Pool, id string) (string, error) {
	query := "SELECT * FROM organizations WHERE id = @id"
	args := pgx.NamedArgs{"id": id}

	row, err := db.Query(context.Background(), query, args)

	if err != nil {
		return "", err
	}

	organization, err := pgx.CollectOneRow(row, pgx.RowToStructByName[organization.Organization])

	if err != nil {
		logger := logger.FromContext(context.Background())
		logger.Error().Err(err).Str("operation", "GetOrganizationIamById").Msg("Error querying database")
		return "", appError.New(appError.InternalError, "Something went wrong", err)
	}
	return organization.IAMOrganizationID, nil
}

// GetOrganizationsByUserSub returns all organizations that a user belongs to,
// based on the user's IAM identifier (sub claim from JWT)
func (c *Client) GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]organization.Organization, error) {
	query := `
		SELECT o.* 
		FROM organizations o
		JOIN users u ON u.organization_id = o.id
		WHERE u.uuid = @userSub`

	args := pgx.NamedArgs{"userSub": userSub}

	rows, err := c.db.Query(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "GetOrganizationsByUserSub").Msg("Error querying database")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	organizations, err := pgx.CollectRows(rows, pgx.RowToStructByName[organization.Organization])

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "GetOrganizationsByUserSub").Msg("Error collecting rows")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	return organizations, nil
}
