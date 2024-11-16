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
