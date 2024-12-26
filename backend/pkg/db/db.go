package db

import (
	"context"
	"planeo/api/pkg/logger"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool" // Standard library bindings for pgx
)

type postgres struct {
	db *pgxpool.Pool
}

func (pg *postgres) Ping(ctx context.Context) error {
	return pg.db.Ping(ctx)
}

func (pg *postgres) Close() {
	pg.db.Close()
}

var (
	pgInstance *postgres
	pgOnce     sync.Once
)

func InitializeDatabase(ctx context.Context, connString string) (*postgres, error) {
	pgOnce.Do(func() {
		db, err := pgxpool.New(ctx, connString)
		if err != nil {
			logger.Error("unable to create connection pool: %s", err.Error())
			panic("Failed to connect to database")
		}

		pgInstance = &postgres{db}
	})

	return pgInstance, nil
}

func GetDatabaseConnection() *pgxpool.Pool {
	return pgInstance.db
}
