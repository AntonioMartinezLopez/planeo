package user

import (
	"planeo/api/internal/resources/user/models"

	"github.com/jmoiron/sqlx"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(database *sqlx.DB) *UserRepository {
	return &UserRepository{
		db: database,
	}
}

func (repo *UserRepository) GetUsersInformation(organizationId string) ([]models.BasicUserInformation, error) {
	query := "SELECT * FROM users WHERE organization = $1"
	users := []models.BasicUserInformation{}
	err := repo.db.Select(&users, query, organizationId)

	if err != nil {
		return nil, err
	}

	return users, nil
}

type DeleteUsersInput struct {
	OrganizationId string
	UserIds        []string
}

func (repo *UserRepository) DeleteUsers(organizationId string, userIds []string) error {

	input := DeleteUsersInput{
		OrganizationId: organizationId,
		UserIds:        userIds,
	}

	// Delete users that are in the organization but not in the list of user IDs
	deleteQuery := `
		DELETE FROM users 
		WHERE organization = :organizationid AND keycloak_id NOT IN (:userids)`

	query, args, _ := sqlx.Named(deleteQuery, input)

	query, args, err := sqlx.In(query, args...)
	if err != nil {
		return err
	}

	query = sqlx.Rebind(sqlx.DOLLAR, query)
	_, err = repo.db.Exec(query, args...)
	if err != nil {
		return err
	}

	return nil
}

type UpdateUserInput struct {
	OrganizationId string
	KeycloakId     string
	Username       string
	FirstName      string
	LastName       string
	Email          string
}

func (repo *UserRepository) SyncUsers(organizationId string, users []models.User) error {
	tx, err := repo.db.Beginx()
	if err != nil {
		return err
	}

	// Step 1: Delete users that are in the organization but not in the list of user IDs
	// Create a list of user IDs
	userIDs := make([]string, len(users))
	for i, user := range users {
		userIDs[i] = user.Id
	}

	input := DeleteUsersInput{
		OrganizationId: organizationId,
		UserIds:        userIDs,
	}

	query, args, _ := sqlx.Named(`
		DELETE FROM users 
		WHERE organization = :organizationid AND keycloak_id NOT IN (:userids)`, input)

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		tx.Rollback()
		return err
	}

	query = sqlx.Rebind(sqlx.DOLLAR, query)
	_, err = tx.Exec(query, args...)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Step 2: Update existing users or insert new users
	for _, user := range users {
		input := UpdateUserInput{
			OrganizationId: organizationId,
			KeycloakId:     user.Id,
			Username:       user.Username,
			FirstName:      user.FirstName,
			LastName:       user.LastName,
			Email:          user.Email,
		}

		// Try to update existing user
		updateQuery := `
			UPDATE users 
			SET username = :username, first_name = :firstname, last_name = :lastname, email = :email 
			WHERE keycloak_id = :keycloakid AND organization = :organizationid`

		result, err := tx.NamedExec(updateQuery, input)
		if err != nil {
			tx.Rollback()
			return err
		}

		// Check if any row was updated
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			tx.Rollback()
			return err
		}

		// If no row was updated, insert new user
		if rowsAffected == 0 {
			insertQuery := `
				INSERT INTO users (username, first_name, last_name, email, keycloak_id, organization) 
				VALUES (:username, :firstname, :lastname, :email, :keycloakid, :organizationid)`
			_, err = tx.NamedExec(insertQuery, input)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit()
}
