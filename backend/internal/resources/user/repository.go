package user

import (
	"context"
	appError "planeo/api/internal/errors"
	"planeo/api/internal/resources/user/models"
	"planeo/api/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(database *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		db: database,
	}
}

func (repo *UserRepository) GetUsersInformation(ctx context.Context, organizationId string) ([]models.BasicUserInformation, error) {
	query := "SELECT * FROM users WHERE organization = @organizationId"
	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := repo.db.Query(ctx, query, args)

	if err != nil {
		appError.New(appError.InternalError, "Something went wrong", err)
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[models.BasicUserInformation])
}

func (repo *UserRepository) UpdateUser(ctx context.Context, organizationId string, userId string, user models.User) error {

	query := `
		UPDATE users 
		SET username = @username, first_name = @firstname, last_name = @lastname, email = @email 
		WHERE iam_user_id = @userID AND organization = @organizationId`

	args := pgx.NamedArgs{
		"organizationId": organizationId,
		"userID":         userId,
		"username":       user.Username,
		"firstname":      user.FirstName,
		"lastname":       user.LastName,
		"email":          user.Email,
	}

	_, err := repo.db.Exec(ctx, query, args)

	if err != nil {
		appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (repo *UserRepository) CreateUser(ctx context.Context, organizationId string, user models.User) error {

	query := `
		INSERT INTO users (username, first_name, last_name, email, iam_user_id, organization) 
		VALUES (@username, @firstname, @lastname, @email, @userID, @organizationId)`

	args := pgx.NamedArgs{
		"organizationId": organizationId,
		"userID":         user.Id,
		"username":       user.Username,
		"firstname":      user.FirstName,
		"lastname":       user.LastName,
		"email":          user.Email,
	}

	_, err := repo.db.Exec(ctx, query, args)

	if err != nil {
		if err == pgx.ErrNoRows {
			return appError.New(appError.EntityNotFound, "User not found in organization")
		}
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (repo *UserRepository) DeleteUser(ctx context.Context, organizationId string, userId string) error {

	query := `
		DELETE FROM users 
		WHERE organization = @organizationId AND iam_user_id = @userId`

	args := pgx.NamedArgs{"organizationId": organizationId, "userId": userId}

	_, err := repo.db.Exec(ctx, query, args)

	if err != nil {
		if err == pgx.ErrNoRows {
			return appError.New(appError.EntityNotFound, "User not found in organization")
		}
		return appError.New(appError.InternalError, "Something went wrong", err)
	}

	return nil
}

func (repo *UserRepository) SyncUsers(ctx context.Context, organizationId string, users []models.User) error {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		appError.New(appError.InternalError, "Something went wrong", err)
	}

	defer tx.Rollback(ctx)

	// Step 1: Delete users that are in the organization but not in the list of the valid user IDs
	// Create a list of user IDs
	userIds := make([]string, len(users))
	for i, user := range users {
		userIds[i] = user.Id
	}

	// Delete users that are in the organization but not in the list of user IDs
	query := `
		DELETE FROM users 
		WHERE organization = @organizationId AND NOT iam_user_id = any(@userIds)`

	args := pgx.NamedArgs{"organizationId": organizationId, "userIds": userIds}

	_, err = tx.Exec(ctx, query, args)

	if err != nil {
		logger.Error("Error deleting in SyncUsers: %s", err.Error())
		tx.Rollback(ctx)
		return appError.New(appError.InternalError, "Error deleting in SyncUsers", err)
	}

	// Step 2: Update existing users or insert new users
	for _, user := range users {
		query := `
		UPDATE users 
		SET username = @username, first_name = @firstname, last_name = @lastname, email = @email 
		WHERE iam_user_id = @userID AND organization = @organizationId`

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
			logger.Error("Error updating user in SyncUsers: %s", err.Error())
			tx.Rollback(ctx)
			return appError.New(appError.InternalError, "Error updating user in SyncUsers", err)
		}
		rowsAffected := result.RowsAffected()

		// If no row was updated, insert new user
		if rowsAffected == 0 {
			query := `
			INSERT INTO users (username, first_name, last_name, email, iam_user_id, organization) 
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
				logger.Error("Error creating user in SyncUsers: %s", err.Error())
				tx.Rollback(ctx)
				return appError.New(appError.InternalError, "Error creating user in SyncUsers", err)
			}
		}
	}

	return tx.Commit(ctx)
}
