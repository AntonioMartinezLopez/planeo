package config

import (
	"fmt"
	"os"
	"planeo/api/pkg/logger"

	"github.com/joho/godotenv"
)

type ApplicationConfiguration struct {
	Host                string
	Port                string
	DbHost              string
	DbPort              string
	DbUser              string
	DbPassword          string
	DbName              string
	KcBaseUrl           string
	KcIssuer            string
	KcOauthClientID     string
	KcAdminClientID     string
	KcAdminClientSecret string
	KcAdminUsername     string
	KcAdminPassword     string
}

var Config *ApplicationConfiguration

func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		logger.Error("Error loading .env file: %v", err)
	}

	Config = &ApplicationConfiguration{
		Host:                readEnvFile("HOST"),
		Port:                readEnvFile("HOST"),
		DbHost:              readEnvFile("DB_HOST"),
		DbPort:              readEnvFile("DB_PORT"),
		DbUser:              readEnvFile("DB_USER"),
		DbPassword:          readEnvFile("DB_PASSWORD"),
		DbName:              readEnvFile("DB_NAME"),
		KcBaseUrl:           readEnvFile("KC_BASE_URL"),
		KcIssuer:            readEnvFile("KC_ISSUER_REALM"),
		KcOauthClientID:     readEnvFile("KC_OAUTH_CLIENT_ID"),
		KcAdminClientID:     readEnvFile("KC_ADMIN_CLIENT_ID"),
		KcAdminClientSecret: readEnvFile("KC_ADMIN_CLIENT_SECRET"),
		KcAdminUsername:     readEnvFile("KC_ADMIN_USERNAME"),
		KcAdminPassword:     readEnvFile("KC_ADMIN_PASSWORD"),
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

func DatabaseConfig() string {
	dbConfig := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", Config.DbUser, Config.DbPassword, Config.DbHost, Config.DbPort, Config.DbName)
	return dbConfig
}

func OauthIssuerUrl() string {
	return fmt.Sprintf("%s/realms/%s", Config.KcBaseUrl, Config.KcIssuer)
}
