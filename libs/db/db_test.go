package db_test

import (
	"context"
	"errors"
	"planeo/libs/db"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:alpine3.20",
		postgres.WithDatabase("db_test"),
		postgres.WithUsername("planeo"),
		postgres.WithPassword("planeo"),
		testcontainers.WithWaitStrategyAndDeadline(5*time.Minute,
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	assert.Nil(t, err)

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	assert.Nil(t, err)

	pool, err := pgxpool.New(ctx, connString)
	assert.Nil(t, err)

	_, err = pool.Exec(ctx, `CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
	assert.Nil(t, err)

	t.Cleanup(func() {
		pool.Close()
		_ = container.Terminate(ctx)
	})

	return pool
}

func TestWithTx(t *testing.T) {
	pool := startTestPool(t)

	t.Run("commits when fn returns nil", func(t *testing.T) {
		err := db.WithTx(context.Background(), pool, func(ctx context.Context) error {
			q := db.FromContext(ctx, pool)
			_, err := q.Exec(ctx, `INSERT INTO widgets (id, name) VALUES (1, 'committed')`)
			return err
		})
		assert.Nil(t, err)

		var name string
		err = pool.QueryRow(context.Background(), `SELECT name FROM widgets WHERE id = 1`).Scan(&name)
		assert.Nil(t, err)
		assert.Equal(t, "committed", name)
	})

	t.Run("rolls back when fn returns an error", func(t *testing.T) {
		sentinel := errors.New("boom")
		err := db.WithTx(context.Background(), pool, func(ctx context.Context) error {
			q := db.FromContext(ctx, pool)
			if _, err := q.Exec(ctx, `INSERT INTO widgets (id, name) VALUES (2, 'rolled-back')`); err != nil {
				return err
			}
			return sentinel
		})
		assert.ErrorIs(t, err, sentinel)

		var count int
		err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM widgets WHERE id = 2`).Scan(&count)
		assert.Nil(t, err)
		assert.Equal(t, 0, count, "a rolled-back transaction must not leave any row behind")
	})
}

func TestFromContext(t *testing.T) {
	pool := startTestPool(t)

	t.Run("returns the pool when ctx carries no transaction", func(t *testing.T) {
		q := db.FromContext(context.Background(), pool)
		assert.Equal(t, pool, q)
	})

	t.Run("returns the transaction when ctx carries one", func(t *testing.T) {
		err := db.WithTx(context.Background(), pool, func(ctx context.Context) error {
			q := db.FromContext(ctx, pool)
			assert.NotEqual(t, pool, q, "FromContext must return the tx, not the pool, once inside WithTx")
			return nil
		})
		assert.Nil(t, err)
	})
}
