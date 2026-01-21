package postgres

import (
	"context"
	"planeo/libs/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	// Database connection and other fields would be defined here
	db *pgxpool.Pool
}

// NewClient creates a new instance of the Postgres client
func NewClient(ctx context.Context, connectionUrl string) *Client {
	db := db.InitializeDatabaseConnection(ctx, connectionUrl)
	return &Client{
		db: db.DB,
	}
}

func (c Client) Close() {
	c.db.Close()
}
