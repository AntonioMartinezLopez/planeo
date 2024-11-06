package db

import (
	"planeo/api/pkg/logger"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // Standard library bindings for pgx
	"github.com/jmoiron/sqlx"
)

var (
	database         *sqlx.DB
	connectionConfig string
	errorCounter     int
)

// function to connect to the database
func connect() error {
	var err error
	database, err = sqlx.Connect("pgx", connectionConfig)
	if err != nil {
		return err
	}
	return nil
}

// function to regulary (every 30s) ping the database to check if the connection is still alive
// If ping failed, connection will be closed
func pingDatabase() {
	for {
		err := database.Ping()
		if err != nil {
			logger.Error("Failed to ping the database: %v", err)
			errorCounter++
			if errorCounter >= 5 {
				panic("Failed to connect to the database after 5 attempts")
			}
		} else {
			if errorCounter > 0 {
				logger.Log("Database connection restored.")
			}
			errorCounter = 0
		}
		time.Sleep(30 * time.Second)
	}
}

func InitializeDatabase(connectionDSN string) {
	logger.Log("Connecting to the database.")
	connectionConfig = connectionDSN
	err := connect()
	if err != nil {
		panic(err)
	}
	logger.Log("Connected to the database.")
	go pingDatabase()
}

// function to get the database connection
func GetDatabaseConnection() *sqlx.DB {
	return database
}
