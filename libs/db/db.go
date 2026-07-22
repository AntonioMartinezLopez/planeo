package db

import (
	"context"
	"planeo/libs/logger"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool" // Standard library bindings for pgx
)

type DBConnection struct {
	DB     *pgxpool.Pool
	cancel context.CancelFunc
}

func (pg *DBConnection) Ping(ctx context.Context) error {
	return pg.DB.Ping(ctx)
}

// Close stops the background ping loop before closing the pool, so Close
// is the only way that loop ever stops - it otherwise runs for the life of
// the process and will panic if a closed pool starts failing pings.
func (pg *DBConnection) Close() {
	pg.cancel()
	pg.DB.Close()
}

func InitializeDatabaseConnection(ctx context.Context, connString string) *DBConnection {

	db, err := pgxpool.New(ctx, connString)
	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Error().Err(err).Msg("unable to create connection pool")
		panic("Failed to connect to database")
	}

	pingCtx, cancel := context.WithCancel(ctx)
	pgInstance := &DBConnection{DB: db, cancel: cancel}

	go pingDatabase(pingCtx, pgInstance)

	return pgInstance
}

func pingDatabase(ctx context.Context, pg *DBConnection) {
	var errorCounter int
	logger := logger.FromContext(ctx)
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := pg.DB.Ping(pingCtx)
			cancel()
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
		}
	}
}

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx, letting
// repository code run the same query methods whether or not it's
// currently inside a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// WithTx runs fn inside a single database transaction on pool, committing
// if fn returns nil and rolling back otherwise. Repository code that calls
// FromContext(ctx, pool) using the ctx passed to fn transparently
// participates in this same transaction.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FromContext returns the pgx.Tx stored in ctx by WithTx, or pool if ctx
// carries no transaction.
func FromContext(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
