package postgres

import (
	"context"
	"planeo/libs/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	// Database connection and other fields would be defined here
	db   *pgxpool.Pool
	conn *db.DBConnection
}

// NewClient creates a new instance of the Postgres client
func NewClient(ctx context.Context, connectionUrl string) *Client {
	conn := db.InitializeDatabaseConnection(ctx, connectionUrl)
	return &Client{
		db:   conn.DB,
		conn: conn,
	}
}

// Close delegates to DBConnection.Close, which stops the background ping
// goroutine before closing the pool - closing c.db directly would leave
// that goroutine running against an already-closed pool.
func (c Client) Close() {
	c.conn.Close()
}
