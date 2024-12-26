package user

import (
	"context"
	"planeo/api/internal/resources/user/models"

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

func (repo *UserRepository) GetUsersInformation(organizationId string) ([]models.BasicUserInformation, error) {
	query := "SELECT * FROM users WHERE organization = @organizationId"
	args := pgx.NamedArgs{"organizationId": organizationId}

	rows, err := repo.db.Query(context.Background(), query, args)

	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[models.BasicUserInformation])
}

func (repo *UserRepository) DeleteUsersNotInList(organizationId string, userIds []string) error {

	// Delete users that are in the organization but not in the list of user IDs
	query := `
		DELETE FROM users 
		WHERE organization = @organizationId AND NOT keycloak_id = any(@userIds)`

	args := pgx.NamedArgs{"organizationId": organizationId, "userIds": userIds}

	_, err := repo.db.Exec(context.Background(), query, args)

	if err != nil {
		return err
	}

	return nil
}

func (repo *UserRepository) UpdateUser(organizationId string, userId string, user models.User) error {

	query := `
		UPDATE users 
		SET username = @username, first_name = @firstname, last_name = @lastname, email = @email 
		WHERE keycloak_id = @keycloakId AND organization = @organizationId`

	args := pgx.NamedArgs{
		"organizationId": organizationId,
		"keycloakId":     userId,
		"username":       user.Username,
		"firstname":      user.FirstName,
		"lastname":       user.LastName,
		"email":          user.Email,
	}

	_, err := repo.db.Exec(context.Background(), query, args)

	if err != nil {
		return err
	}

	return nil
}

func (repo *UserRepository) CreateUser(organizationId string, user models.User) error {

	query := `
		INSERT INTO users (username, first_name, last_name, email, keycloak_id, organization) 
		VALUES (@username, @firstname, @lastname, @email, @keycloakId, @organizationId)`

	args := pgx.NamedArgs{
		"organizationId": organizationId,
		"keycloakId":     user.Id,
		"username":       user.Username,
		"firstname":      user.FirstName,
		"lastname":       user.LastName,
		"email":          user.Email,
	}

	_, err := repo.db.Exec(context.Background(), query, args)
	if err != nil {
		return err
	}

	return nil
}

func (repo *UserRepository) DeleteUser(organizationId string, userId string) error {

	query := `
		DELETE FROM users 
		WHERE organization = @organizationId AND keycloak_id = @keycloakId`

	args := pgx.NamedArgs{"organizationId": organizationId, "keycloakId": userId}

	_, err := repo.db.Exec(context.Background(), query, args)
	if err != nil {
		return err
	}

	return nil
}

func (repo *UserRepository) SyncUsers(organizationId string, users []models.User) error {
	tx, err := repo.db.Begin(context.Background())
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())

	// Step 1: Delete users that are in the organization but not in the list of the valid user IDs
	// Create a list of user IDs
	userIds := make([]string, len(users))
	for i, user := range users {
		userIds[i] = user.Id
	}

	// Delete users that are in the organization but not in the list of user IDs
	query := `
		DELETE FROM users 
		WHERE organization = @organizationId AND NOT keycloak_id = any(@userIds)`

	args := pgx.NamedArgs{"organizationId": organizationId, "userIds": userIds}

	_, err = tx.Exec(context.Background(), query, args)

	if err != nil {
		tx.Rollback(context.Background())
		return err
	}

	// Step 2: Update existing users or insert new users
	for _, user := range users {
		query := `
		UPDATE users 
		SET username = @username, first_name = @firstname, last_name = @lastname, email = @email 
		WHERE keycloak_id = @keycloakId AND organization = @organizationId`

		args := pgx.NamedArgs{
			"organizationId": organizationId,
			"keycloakId":     user.Id,
			"username":       user.Username,
			"firstname":      user.FirstName,
			"lastname":       user.LastName,
			"email":          user.Email,
		}

		result, err := tx.Exec(context.Background(), query, args)

		if err != nil {
			tx.Rollback(context.Background())
			return err
		}
		rowsAffected := result.RowsAffected()

		// If no row was updated, insert new user
		if rowsAffected == 0 {
			query := `
			INSERT INTO users (username, first_name, last_name, email, keycloak_id, organization) 
			VALUES (@username, @firstname, @lastname, @email, @keycloakId, @organizationId)`

			args := pgx.NamedArgs{
				"organizationId": organizationId,
				"keycloakId":     user.Id,
				"username":       user.Username,
				"firstname":      user.FirstName,
				"lastname":       user.LastName,
				"email":          user.Email,
			}

			_, err := tx.Exec(context.Background(), query, args)
			if err != nil {
				tx.Rollback(context.Background())
				return err
			}
		}
	}

	return tx.Commit(context.Background())
}
