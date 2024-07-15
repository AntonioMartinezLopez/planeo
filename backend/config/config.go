package config

import (
	"fmt"
	"os"
	"planeo/api/pkg/logger"

	"github.com/joho/godotenv"
)

var variables = []string{"HOST", "PORT", "JWKS_URL"}

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

func ServerConfig() string {
	appServerUrl := fmt.Sprintf("%s:%s", os.Getenv("HOST"), os.Getenv("PORT"))
	logger.Log("Server Running at %s", appServerUrl)
	return appServerUrl
}
