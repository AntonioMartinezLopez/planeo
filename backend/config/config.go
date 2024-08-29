package config

import (
	"fmt"
	"os"
	"planeo/api/pkg/logger"

	"github.com/joho/godotenv"
)

type ApplicationConfiguration struct {
	Host              string
	Port              string
	OAuthIssuer       string
	OAuthClientID     string
	OAuthClientSecret string
}

var Config *ApplicationConfiguration

func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		logger.Error("Error loading .env file: %v", err)
	}

	Config = &ApplicationConfiguration{
		Host:              readEnvFile("HOST"),
		Port:              readEnvFile("HOST"),
		OAuthIssuer:       readEnvFile("OAUTH_ISSUER"),
		OAuthClientID:     readEnvFile("OAUTH_CLIENT_ID"),
		OAuthClientSecret: readEnvFile("OAUTH_CLIENT_SECRET"),
	}
}

func readEnvFile(envName string) string {
	envVariable, envExists := os.LookupEnv(envName)
	if !envExists {
		logger.Fatal("Missing env variable '%s'. Aborting...\n", envName)
	}

	return envVariable
}

func ServerConfig() string {
	appServerUrl := fmt.Sprintf("%s:%s", os.Getenv("HOST"), os.Getenv("PORT"))
	return appServerUrl
}

func init() {
	LoadConfig()
}
