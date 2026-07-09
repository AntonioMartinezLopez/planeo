package config

import (
	"context"
	"fmt"
	"os"
	"planeo/libs/logger"

	"github.com/joho/godotenv"
)

type ApplicationConfiguration struct {
	Host            string
	Port            string
	KafkaBrokers    string
	DbHost          string
	DbPort          string
	DbUser          string
	DbPassword      string
	DbName          string
	KcBaseUrl       string
	KcIssuer        string
	KcOauthClientID string
}

func (c *ApplicationConfiguration) ServerConfig() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

func (c *ApplicationConfiguration) DatabaseConfig() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DbUser, c.DbPassword, c.DbHost, c.DbPort, c.DbName)
}

func (c *ApplicationConfiguration) OauthIssuerUrl() string {
	return fmt.Sprintf("%s/realms/%s", c.KcBaseUrl, c.KcIssuer)
}

func LoadConfig(ctx context.Context, filenames ...string) *ApplicationConfiguration {
	err := godotenv.Load(filenames...)
	if err != nil {
		logger := logger.FromContext(ctx)
		logger.Warn().Err(err).Msg("Error loading .env file")
	}

	return &ApplicationConfiguration{
		Host:            readEnvVariable(ctx, "HOST"),
		Port:            readEnvVariable(ctx, "PORT"),
		KafkaBrokers:    readEnvVariable(ctx, "KAFKA_BROKERS"),
		DbHost:          readEnvVariable(ctx, "DB_HOST"),
		DbPort:          readEnvVariable(ctx, "DB_PORT"),
		DbUser:          readEnvVariable(ctx, "DB_USER"),
		DbPassword:      readEnvVariable(ctx, "DB_PASSWORD"),
		DbName:          readEnvVariable(ctx, "DB_NAME"),
		KcBaseUrl:       readEnvVariable(ctx, "KC_BASE_URL"),
		KcIssuer:        readEnvVariable(ctx, "KC_ISSUER_REALM"),
		KcOauthClientID: readEnvVariable(ctx, "KC_OAUTH_CLIENT_ID"),
	}
}

func readEnvVariable(ctx context.Context, name string) string {
	v, ok := os.LookupEnv(name)
	if !ok {
		logger := logger.FromContext(ctx)
		logger.Fatal().Msgf("Missing env variable '%s'. Aborting...\n", name)
	}
	return v
}
