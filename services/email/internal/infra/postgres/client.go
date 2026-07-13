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
