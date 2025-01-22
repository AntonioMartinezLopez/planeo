package utils

import (
	"context"
	"path/filepath"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func StartPostgresContainer(ctx context.Context) (*postgres.PostgresContainer, error) {
	return postgres.Run(ctx,
		"postgres:alpine3.20",
		postgres.WithInitScripts(filepath.Join("../../../db/migrations")),
		postgres.WithDatabase("planeo"),
		postgres.WithUsername("planeo"),
		postgres.WithPassword("planeo"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
}
