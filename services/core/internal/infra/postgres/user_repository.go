package postgres

import (
	"context"
	"fmt"
	"planeo/services/core/internal/domain/user"

	"github.com/jackc/pgx/v5"
)

func (c *Client) GetIamOrganizationIdentifier(ctx context.Context, organizationId int) (string, error) {
	query := "SELECT iam_organization_id FROM organizations WHERE id = @organizationId LIMIT 1"
	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := c.db.Query(ctx, query, args)
	if err != nil {
		return "", NewDatabaseError("error querying database", err)
	}

	type Organization struct {
		IamOrganizationID string `json:"iam_organization_id" db:"iam_organization_id"`
	}
	organization := Organization{}

	if rows.Next() {
		err = rows.Scan(&organization.IamOrganizationID)
		if err != nil {
			return "", NewDatabaseError("error scanning organization IAM ID", err)
		}
	}
	rows.Close()

	if err != nil {
		return "", NewDatabaseError("error retrieving organization IAM ID", err)
	}
	return organization.IamOrganizationID, nil
}

func (c *Client) GetUsers(ctx context.Context, organizationId int) ([]user.User, error) {
	query := "SELECT * FROM users WHERE organization_id = @organizationId"
	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := c.db.Query(ctx, query, args)
	if err != nil {
		return nil, NewDatabaseError("error querying database", err)
	}

	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[user.User])
	if err != nil {
		return nil, NewDatabaseError("error collecting users", err)
	}

	return users, nil
}

func (c *Client) UpdateUser(ctx context.Context, organizationId int, uuid string, updateUser user.UpdateUser) error {
	query := `
		UPDATE users 
		SET username = @username, first_name = @firstname, last_name = @lastname, email = @email 
		WHERE uuid = @uuid AND organization_id = @organizationId`

	args := pgx.NamedArgs{
		"organizationId": organizationId,
		"uuid":           uuid,
		"username":       updateUser.Username,
		"firstname":      updateUser.FirstName,
		"lastname":       updateUser.LastName,
		"email":          updateUser.Email,
	}

	_, err := c.db.Exec(ctx, query, args)
	if err != nil {
		if err == pgx.ErrNoRows {
			return user.UserNotFoundError
		}
		return NewDatabaseError("error updating user", err)
	}

	return nil
}

func (c *Client) CreateUser(ctx context.Context, organizationId int, uuid string, user user.NewUser) error {
	query := `
		INSERT INTO users (username, first_name, last_name, email, uuid, organization_id) 
		VALUES (@username, @firstname, @lastname, @email, @uuid, @organizationId)`

	args := pgx.NamedArgs{
		"organizationId": organizationId,
		"uuid":           uuid,
		"username":       user.Username,
		"firstname":      user.FirstName,
		"lastname":       user.LastName,
		"email":          user.Email,
	}

	_, err := c.db.Exec(ctx, query, args)
	if err != nil {
		return NewDatabaseError("error creating user", err)
	}

	return nil
}

func (c *Client) DeleteUser(ctx context.Context, organizationId int, uuid string) error {
	query := `
		DELETE FROM users 
		WHERE organization_id = @organizationId AND uuid = @uuid`

	args := pgx.NamedArgs{"organizationId": organizationId, "uuid": uuid}

	_, err := c.db.Exec(ctx, query, args)
	if err != nil {
		if err == pgx.ErrNoRows {
			return user.UserNotFoundError
		}

		return NewDatabaseError(fmt.Sprintf("error deleting user with uuid %s", uuid), err)
	}

	return nil
}

//nolint:funlen
func (c *Client) SyncUsers(ctx context.Context, organizationId int, users []user.IAMUser) error {
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	// Step 1: Delete users that are in the organization but not in the list of the valid user IDs
	// Create a list of user IDs
	userUuids := make([]string, len(users))
	for i, user := range users {
		userUuids[i] = user.Uuid
	}

	// Delete users that are in the organization but not in the list of user IDs
	query := `
		DELETE FROM users 
		WHERE organization_id = @organizationId AND NOT uuid = any(@uuids)`

	args := pgx.NamedArgs{"organizationId": organizationId, "uuids": userUuids}

	_, err = tx.Exec(ctx, query, args)
	if err != nil {
		_ = tx.Rollback(ctx)
		return NewDatabaseError("failed to delete users not in IAM", err)
	}

	// Step 2: Update existing users or insert new users
	for _, user := range users {
		query := `
		UPDATE users 
		SET username = @username, first_name = @firstname, last_name = @lastname, email = @email 
		WHERE uuid = @uuid AND organization_id = @organizationId`

		args := pgx.NamedArgs{
			"organizationId": organizationId,
			"uuid":           user.Uuid,
			"username":       user.Username,
			"firstname":      user.FirstName,
			"lastname":       user.LastName,
			"email":          user.Email,
		}

		result, err := tx.Exec(ctx, query, args)

		if err != nil {
			_ = tx.Rollback(ctx)
			return NewDatabaseError("failed to update user", err)
		}
		rowsAffected := result.RowsAffected()

		// If no row was updated, insert new user
		if rowsAffected == 0 {
			query := `
			INSERT INTO users (username, first_name, last_name, email, uuid, organization_id) 
			VALUES (@username, @firstname, @lastname, @email, @uuid, @organizationId)`

			args := pgx.NamedArgs{
				"organizationId": organizationId,
				"uuid":           user.Uuid,
				"username":       user.Username,
				"firstname":      user.FirstName,
				"lastname":       user.LastName,
				"email":          user.Email,
			}

			_, err := tx.Exec(ctx, query, args)
			if err != nil {
				_ = tx.Rollback(ctx)
				return NewDatabaseError("failed to insert user", err)
			}
		}
	}

	return tx.Commit(ctx)
}
