package db

import (
	"context"
	"planeo/api/pkg/logger"
	"sync"
	"time"

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
	pgInstance   *postgres
	pgOnce       sync.Once
	errorCounter int
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

	go pingDatabase()

	return pgInstance, nil
}

func pingDatabase() {

	for {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := pgInstance.db.Ping(ctx)
			if err != nil {
				logger.Error("Failed to ping the database: %v", err)
				errorCounter++
				if errorCounter >= 5 {
					panic("Failed to connect to the database after 5 attempts")
				}
			} else {
				if errorCounter > 0 {
					logger.Log("Database connection restored.")
				}
				errorCounter = 0
			}
		}()
		time.Sleep(20 * time.Second)
	}
}

func GetDatabaseConnection() *pgxpool.Pool {
	return pgInstance.db
}
