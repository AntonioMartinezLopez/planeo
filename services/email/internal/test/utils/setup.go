package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"planeo/services/email/internal/config"
	"planeo/services/email/internal/infra/postgres"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	postgresContainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type IntegrationTestEnvironment struct {
	PostgresContainer *postgresContainer.PostgresContainer
	Configuration     *config.ApplicationConfiguration
	DB                *postgres.Client
	// Pool is the same underlying connection pool as DB, exposed directly
	// for tests that need to assert on database state outside the
	// Repository port (e.g. counting rows a repository method doesn't
	// itself report back).
	Pool *pgxpool.Pool
}

func NewIntegrationTestEnvironment(t *testing.T) *IntegrationTestEnvironment {
	postgresCont, err := StartPostgresContainer(context.Background())
	if err != nil {
		panic(err)
	}

	postgresPort, err := postgresCont.MappedPort(context.Background(), "5432")
	if err != nil {
		t.Error(err)
	}

	cfg := config.LoadConfig(context.Background(), "../../../.env.test.template")
	cfg.DbPort = postgresPort.Port()

	env := &IntegrationTestEnvironment{
		PostgresContainer: postgresCont,
		Configuration:     cfg,
	}

	if err := env.MigrateDatabase(false); err != nil {
		t.Error(err.Error())
		panic(err)
	}

	db := postgres.NewClient(context.Background(), cfg.DatabaseConfig())
	env.DB = db
	env.Pool = db.Pool()

	t.Cleanup(func() {
		ctx := context.Background()
		if err := env.PostgresContainer.Terminate(ctx); err != nil {
			panic(err)
		}
		env.DB.Close()
	})

	return env
}

func (env *IntegrationTestEnvironment) MigrateDatabase(tearDown bool) error {
	operation := "up"
	if tearDown {
		operation = "down"
	}

	migrationsDir, _ := filepath.Abs(filepath.Join("..", "..", "..", "internal", "infra", "postgres", "migrations"))
	cmd := exec.Command("goose", "-dir", migrationsDir, "postgres", fmt.Sprintf("postgres://planeo:planeo@127.0.0.1:%s/mail?sslmode=disable",
		env.Configuration.DbPort), operation)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}
	return nil
}
