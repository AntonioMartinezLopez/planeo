package config

import (
	"os"
	"planeo/api/pkg/logger"

	"github.com/joho/godotenv"
)

var variables = []string{"PORT"}

func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		logger.Error("Error loading .env file: %v", err)
	}

	for _, env := range variables {
		_, envExists := os.LookupEnv(env)
		if !envExists {
			logger.Fatal("Missing env variable '%s'. Aborting...\n", env)
		}
	}
}
