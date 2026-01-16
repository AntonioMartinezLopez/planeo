package config

import (
	"context"
	"fmt"
	"os"
	"planeo/libs/logger"

	"github.com/joho/godotenv"
)

type ApplicationConfiguration struct {
	Host                string
	Port                string
	NatsUrl             string
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

func LoadConfig(ctx context.Context, filenames ...string) *ApplicationConfiguration {

	err := godotenv.Load(filenames...)
	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Warn().Err(err).Msg("Error loading .env file")
	}

	config := &ApplicationConfiguration{
		Host:                readEnvVariable(ctx, "HOST"),
		Port:                readEnvVariable(ctx, "PORT"),
		NatsUrl:             readEnvVariable(ctx, "NATS_URL"),
		DbHost:              readEnvVariable(ctx, "DB_HOST"),
		DbPort:              readEnvVariable(ctx, "DB_PORT"),
		DbUser:              readEnvVariable(ctx, "DB_USER"),
		DbPassword:          readEnvVariable(ctx, "DB_PASSWORD"),
		DbName:              readEnvVariable(ctx, "DB_NAME"),
		KcBaseUrl:           readEnvVariable(ctx, "KC_BASE_URL"),
		KcIssuer:            readEnvVariable(ctx, "KC_ISSUER_REALM"),
		KcOauthClientID:     readEnvVariable(ctx, "KC_OAUTH_CLIENT_ID"),
		KcAdminClientID:     readEnvVariable(ctx, "KC_ADMIN_CLIENT_ID"),
		KcAdminClientSecret: readEnvVariable(ctx, "KC_ADMIN_CLIENT_SECRET"),
		KcAdminUsername:     readEnvVariable(ctx, "KC_ADMIN_USERNAME"),
		KcAdminPassword:     readEnvVariable(ctx, "KC_ADMIN_PASSWORD"),
	}

	return config
}

func readEnvVariable(ctx context.Context, envName string) string {
	envVariable, envExists := os.LookupEnv(envName)
	if !envExists {
		logger := logger.FromContext(ctx)
		logger.Fatal().Msgf("Missing env variable '%s'. Aborting...\n", envName)
	}

	return envVariable
}
