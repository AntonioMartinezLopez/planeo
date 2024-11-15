package user

import (
	"context"

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

func (repo *UserRepository) GetUsersInformation(ctx context.Context, organizationId string) ([]BasicUserInformation, error) {
	query := "SELECT * FROM users WHERE organization = $1"
	users := []BasicUserInformation{}
	err := repo.db.SelectContext(ctx, &users, query, organizationId)

	if err != nil {
		return nil, err
	}

	return users, nil
}
