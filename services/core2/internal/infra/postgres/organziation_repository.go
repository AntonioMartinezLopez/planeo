package postgres

import (
	"context"
	"planeo/services/core2/internal/domain/organization"

	"github.com/jackc/pgx/v5"
)

func (c *Client) GetOrganizationById(ctx context.Context, id int) (organization.Organization, error) {
	query := "SELECT * FROM organizations WHERE id = @id"
	args := pgx.NamedArgs{"id": id}

	row, err := c.db.Query(ctx, query, args)
	if err != nil {
		return organization.Organization{}, NewDatabaseError("error querying database", err)
	}

	org, err := pgx.CollectOneRow(row, pgx.RowToStructByName[organization.Organization])
	if err != nil {
		return organization.Organization{}, NewDatabaseError("error collecting organization", err)
	}

	return org, nil
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
		return nil, NewDatabaseError("error querying database", err)
	}

	organizations, err := pgx.CollectRows(rows, pgx.RowToStructByName[organization.Organization])
	if err != nil {
		return nil, NewDatabaseError("error collecting organizations", err)
	}

	return organizations, nil
}
