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
		Host:                readEnvVariable("HOST"),
		Port:                readEnvVariable("PORT"),
		DbHost:              readEnvVariable("DB_HOST"),
		DbPort:              readEnvVariable("DB_PORT"),
		DbUser:              readEnvVariable("DB_USER"),
		DbPassword:          readEnvVariable("DB_PASSWORD"),
		DbName:              readEnvVariable("DB_NAME"),
		KcBaseUrl:           readEnvVariable("KC_BASE_URL"),
		KcIssuer:            readEnvVariable("KC_ISSUER_REALM"),
		KcOauthClientID:     readEnvVariable("KC_OAUTH_CLIENT_ID"),
		KcAdminClientID:     readEnvVariable("KC_ADMIN_CLIENT_ID"),
		KcAdminClientSecret: readEnvVariable("KC_ADMIN_CLIENT_SECRET"),
		KcAdminUsername:     readEnvVariable("KC_ADMIN_USERNAME"),
		KcAdminPassword:     readEnvVariable("KC_ADMIN_PASSWORD"),
	}

	return config
}

func readEnvVariable(envName string) string {
	envVariable, envExists := os.LookupEnv(envName)
	if !envExists {
		logger.Fatal("Missing env variable '%s'. Aborting...\n", envName)
	}

	return envVariable
}
