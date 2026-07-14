package main

import (
	"context"
	"fmt"
	"os"
	"planeo/libs/logger"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DbHost       string
	DbPort       string
	DbUser       string
	DbPassword   string
	DbName       string
	KafkaBrokers string
	PollInterval time.Duration
	BatchSize    int
	MaxAttempts  int
	ClaimTTL     time.Duration
}

func (c *Config) DatabaseConfig() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DbUser, c.DbPassword, c.DbHost, c.DbPort, c.DbName)
}

func LoadConfig(ctx context.Context, filenames ...string) *Config {
	if err := godotenv.Load(filenames...); err != nil {
		l := logger.FromContext(ctx)
		l.Warn().Err(err).Msg("Error loading .env file")
	}

	return &Config{
		DbHost:       readEnvVariable(ctx, "DB_HOST"),
		DbPort:       readEnvVariable(ctx, "DB_PORT"),
		DbUser:       readEnvVariable(ctx, "DB_USER"),
		DbPassword:   readEnvVariable(ctx, "DB_PASSWORD"),
		DbName:       readEnvVariable(ctx, "DB_NAME"),
		KafkaBrokers: readEnvVariable(ctx, "KAFKA_BROKERS"),
		PollInterval: readDurationEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_POLL_INTERVAL", 1*time.Second),
		BatchSize:    readIntEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_BATCH_SIZE", 100),
		MaxAttempts:  readIntEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_MAX_ATTEMPTS", 5),
		ClaimTTL:     readDurationEnvVariable(ctx, "EMAIL_RECEIVED_PRODUCER_CLAIM_TTL", 30*time.Second),
	}
}

func readEnvVariable(ctx context.Context, name string) string {
	v, ok := os.LookupEnv(name)
	if !ok {
		l := logger.FromContext(ctx)
		l.Fatal().Msgf("Missing env variable '%s'. Aborting...\n", name)
	}
	return v
}

func readDurationEnvVariable(ctx context.Context, name string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Fatal().Err(err).Msgf("Invalid duration for env variable '%s'", name)
	}
	return d
}

func readIntEnvVariable(ctx context.Context, name string, def int) int {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l := logger.FromContext(ctx)
		l.Fatal().Err(err).Msgf("Invalid integer for env variable '%s'", name)
	}
	return n
}
