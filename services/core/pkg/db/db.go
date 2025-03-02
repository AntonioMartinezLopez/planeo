package db

import (
	"context"
	"planeo/services/core/pkg/logger"
	"time"

	"github.com/jackc/pgx/v5/pgxpool" // Standard library bindings for pgx
)

type DBConnection struct {
	DB *pgxpool.Pool
}

func (pg *DBConnection) Ping(ctx context.Context) error {
	return pg.DB.Ping(ctx)
}

func (pg *DBConnection) Close() {
	pg.DB.Close()
}

func InitializeDatabaseConnection(ctx context.Context, connString string) *DBConnection {

	db, err := pgxpool.New(ctx, connString)
	if err != nil {
		logger.Error("unable to create connection pool: %s", err.Error())
		panic("Failed to connect to database")
	}

	pgInstance := &DBConnection{db}

	go pingDatabase(pgInstance)

	return pgInstance
}

func pingDatabase(pg *DBConnection) {
	var errorCounter int
	for {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := pg.DB.Ping(ctx)
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
