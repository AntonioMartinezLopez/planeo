package postgres

import (
	"context"
	"planeo/libs/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	db   *pgxpool.Pool
	conn *db.DBConnection
}

func NewClient(ctx context.Context, connString string) *Client {
	conn := db.InitializeDatabaseConnection(ctx, connString)
	return &Client{db: conn.DB, conn: conn}
}

// Close delegates to DBConnection.Close, which stops the background ping
// goroutine before closing the pool - closing c.db directly would leave
// that goroutine running against an already-closed pool.
func (c *Client) Close() {
	c.conn.Close()
}

// Pool exposes the underlying connection pool for callers that need to run
// queries outside the Repository port (e.g. integration tests asserting on
// database state directly).
func (c *Client) Pool() *pgxpool.Pool {
	return c.db
}
