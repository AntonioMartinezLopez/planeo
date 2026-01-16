package postgres

import (
	"context"
	appError "planeo/libs/errors"
	"planeo/libs/logger"
	"planeo/services/core2/internal/domain/user"

	"github.com/jackc/pgx/v5"
)

func (c *Client) GetIamOrganizationIdentifier(ctx context.Context, organizationId int) (string, error) {

	query := "SELECT iam_organization_id FROM organizations WHERE id = @organizationId LIMIT 1"
	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := c.db.Query(ctx, query, args)

	if err != nil {
		return "", err
	}

	type Organization struct {
		IamOrganizationID string `json:"iam_organization_id" db:"iam_organization_id"`
	}
	organization := Organization{}

	if rows.Next() {
		err = rows.Scan(&organization.IamOrganizationID)
		if err != nil {
			logger := logger.FromContext(ctx)
			logger.Error().Err(err).Str("operation", "GetIamOrganizationIdentifier").Msg("Error scanning row")
			return "", err
		}
	}
	rows.Close()

	if err != nil {
		return "", err
	}
	return organization.IamOrganizationID, nil
}

func (c *Client) GetUsersInformation(ctx context.Context, organizationId int) ([]user.User, error) {
	query := "SELECT * FROM users WHERE organization_id = @organizationId"
	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := c.db.Query(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "GetUsersInformation").Msg("Error querying database")
		return nil, appError.New(appError.InternalError, "Something went wrong", err)
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[user.User])
}

func (c *Client) UpdateUser(ctx context.Context, organizationId int, userId string, user user.User) error {

	query := `
		UPDATE users 
		SET username = @username, first_name = @firstname, last_name = @lastname, email = @email 
		WHERE iam_user_id = @userID AND organization_id = @organizationId`

	args := pgx.NamedArgs{
		"organizationId": organizationId,
		"userID":         userId,
		"username":       user.Username,
		"firstname":      user.FirstName,
		"lastname":       user.LastName,
		"email":          user.Email,
	}

	_, err := c.db.Exec(ctx, query, args)

	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "UpdateUser").Msg("Error updating user")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (c *Client) CreateUser(ctx context.Context, organizationId int, user user.User) error {

	query := `
		INSERT INTO users (username, first_name, last_name, email, iam_user_id, organization_id) 
		VALUES (@username, @firstname, @lastname, @email, @userID, @organizationId)`

	args := pgx.NamedArgs{
		"organizationId": organizationId,
		"userID":         user.Id,
		"username":       user.Username,
		"firstname":      user.FirstName,
		"lastname":       user.LastName,
		"email":          user.Email,
	}

	_, err := c.db.Exec(ctx, query, args)

	if err != nil {
		if err == pgx.ErrNoRows {
			return appError.New(appError.EntityNotFound, "User not found in organization")
		}
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "CreateUser").Msg("Error creating user")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (c *Client) DeleteUser(ctx context.Context, organizationId int, userId string) error {

	query := `
		DELETE FROM users 
		WHERE organization_id = @organizationId AND iam_user_id = @userId`

	args := pgx.NamedArgs{"organizationId": organizationId, "userId": userId}

	_, err := c.db.Exec(ctx, query, args)

	if err != nil {
		if err == pgx.ErrNoRows {
			return appError.New(appError.EntityNotFound, "User not found in organization")
		}
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Str("operation", "DeleteUser").Msg("Error deleting user")
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

//nolint:funlen
func (c *Client) SyncUsers(ctx context.Context, organizationId int, users []user.User) error {
	logger := logger.FromContext(ctx)
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	// Step 1: Delete users that are in the organization but not in the list of the valid user IDs
	// Create a list of user IDs
	userIds := make([]string, len(users))
	for i, user := range users {
		userIds[i] = user.IAMUserID
	}

	// Delete users that are in the organization but not in the list of user IDs
	query := `
		DELETE FROM users 
		WHERE organization_id = @organizationId AND NOT iam_user_id = any(@userIds)`

	args := pgx.NamedArgs{"organizationId": organizationId, "userIds": userIds}

	_, err = tx.Exec(ctx, query, args)

	if err != nil {
		logger.Error().Err(err).Msg("Error deleting in SyncUsers.")
		_ = tx.Rollback(ctx)
		return appError.New(appError.InternalError, "Error deleting in SyncUsers", err)
	}

	// Step 2: Update existing users or insert new users
	for _, user := range users {
		query := `
		UPDATE users 
		SET username = @username, first_name = @firstname, last_name = @lastname, email = @email 
		WHERE iam_user_id = @userID AND organization_id = @organizationId`

		args := pgx.NamedArgs{
			"organizationId": organizationId,
			"userID":         user.Id,
			"username":       user.Username,
			"firstname":      user.FirstName,
			"lastname":       user.LastName,
			"email":          user.Email,
		}

		result, err := tx.Exec(ctx, query, args)

		if err != nil {
			logger.Error().Err(err).Msg("Error updating user in SyncUsers")
			_ = tx.Rollback(ctx)
			return appError.New(appError.InternalError, "Error updating user in SyncUsers", err)
		}
		rowsAffected := result.RowsAffected()

		// If no row was updated, insert new user
		if rowsAffected == 0 {
			query := `
			INSERT INTO users (username, first_name, last_name, email, iam_user_id, organization_id) 
			VALUES (@username, @firstname, @lastname, @email, @userID, @organizationId)`

			args := pgx.NamedArgs{
				"organizationId": organizationId,
				"userID":         user.Id,
				"username":       user.Username,
				"firstname":      user.FirstName,
				"lastname":       user.LastName,
				"email":          user.Email,
			}

			_, err := tx.Exec(ctx, query, args)
			if err != nil {
				logger.Error().Err(err).Msg("Error creating user in SyncUsers.")
				_ = tx.Rollback(ctx)
				return appError.New(appError.InternalError, "Error creating user in SyncUsers", err)
			}
		}
	}

	return tx.Commit(ctx)
}
