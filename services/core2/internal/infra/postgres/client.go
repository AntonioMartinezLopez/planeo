package postgres

import (
	"planeo/libs/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	// Database connection and other fields would be defined here
	db *pgxpool.Pool
}

// NewClient creates a new instance of the Postgres client
func NewClient(db *db.DBConnection) *Client {
	return &Client{
		db: db.DB,
	}
}

func (c Client) Close() {
	c.db.Close()
}
