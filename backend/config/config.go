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

func (config *ApplicationConfiguration) ServerConfig() string {
	appServerUrl := fmt.Sprintf("%s:%s", config.Host, config.Port)
	return appServerUrl
}

func (config *ApplicationConfiguration) DatabaseConfig() string {
	dbConfig := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", config.DbUser, config.DbPassword, config.DbHost, config.DbPort, config.DbName)
	return dbConfig
}

func (config *ApplicationConfiguration) OauthIssuerUrl() string {
	return fmt.Sprintf("%s/realms/%s", config.KcBaseUrl, config.KcIssuer)
}

func LoadConfig(filenames ...string) *ApplicationConfiguration {
	err := godotenv.Load(filenames...)
	if err != nil {
		logger.Error("Error loading .env file: %v", err)
	}

	config := &ApplicationConfiguration{
		Host:                readEnvFile("HOST"),
		Port:                readEnvFile("PORT"),
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

	return config
}

func readEnvFile(envName string) string {
	envVariable, envExists := os.LookupEnv(envName)
	if !envExists {
		logger.Fatal("Missing env variable '%s'. Aborting...\n", envName)
	}

	return envVariable
}
