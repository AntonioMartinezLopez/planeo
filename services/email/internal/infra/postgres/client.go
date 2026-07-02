// services/email/internal/infra/postgres/client.go
package postgres

import (
	"context"
	"planeo/libs/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	db *pgxpool.Pool
}

func NewClient(ctx context.Context, connString string) *Client {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Fatal().Err(err).Msg("unable to create connection pool")
	}
	return &Client{db: pool}
}

func (c *Client) Close() {
	c.db.Close()
}
