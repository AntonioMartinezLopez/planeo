package db

import (
	"context"
	"planeo/libs/logger"
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
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Msg("unable to create connection pool")
		panic("Failed to connect to database")
	}

	pgInstance := &DBConnection{db}

	go pingDatabase(ctx, pgInstance)

	return pgInstance
}

func pingDatabase(ctx context.Context, pg *DBConnection) {
	var errorCounter int
	logger := logger.FromContext(ctx)
	for {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := pg.DB.Ping(ctx)
			if err != nil {

				logger.Error().Err(err).Msg("Failed to ping the database")
				errorCounter++
				if errorCounter >= 5 {
					panic("Failed to connect to the database after 5 attempts")
				}
			} else {
				if errorCounter > 0 {
					logger.Info().Msg("Database connection restored.")
				}
				errorCounter = 0
			}
		}()
		time.Sleep(20 * time.Second)
	}
}
