package postgres

import (
	"context"
	"planeo/libs/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	db *pgxpool.Pool
}

func NewClient(ctx context.Context, connString string) *Client {
	conn := db.InitializeDatabaseConnection(ctx, connString)
	return &Client{db: conn.DB}
}

func (c *Client) Close() {
	c.db.Close()
}

// Pool exposes the underlying connection pool for callers that need to run
// queries outside the Repository port (e.g. integration tests asserting on
// database state directly).
func (c *Client) Pool() *pgxpool.Pool {
	return c.db
}
