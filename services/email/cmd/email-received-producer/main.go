package main

import (
	"context"
	"os/signal"
	"planeo/libs/logger"
	"planeo/libs/outbox"
	"planeo/services/email/internal/infra/postgres"
	"strings"
	"syscall"
)

func main() {
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	log := logger.New("email-received-producer")
	ctx := logger.WithContext(context.Background(), log)

	log.Info().Msg("Loading environment variables")
	cfg := LoadConfig(ctx)

	db := postgres.NewClient(ctx, cfg.DatabaseConfig())
	defer db.Close()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	producer, kafkaClient, err := outbox.NewProducer(brokers)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}
	defer kafkaClient.Close()

	relay := outbox.NewRelay(db, producer,
		outbox.WithPollInterval(cfg.PollInterval),
		outbox.WithBatchSize(cfg.BatchSize),
		outbox.WithMaxAttempts(cfg.MaxAttempts),
		outbox.WithClaimTTL(cfg.ClaimTTL),
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Outbox relay running")
	if err := relay.Run(runCtx); err != nil {
		log.Info().Err(err).Msg("Outbox relay stopped")
	}
}
