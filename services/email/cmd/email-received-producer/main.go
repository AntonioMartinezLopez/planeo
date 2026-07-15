package main

import (
	"context"
	"os/signal"
	"planeo/libs/events/contracts"
	"planeo/libs/logger"
	"planeo/libs/outbox"
	emailoutbox "planeo/services/email/internal/infra/outbox"
	"planeo/services/email/internal/infra/postgres"
	"strings"
	"syscall"

	"github.com/google/uuid"
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

	adapter := emailoutbox.NewEmailReceivedProducerAdapter(
		db, producer, contracts.EmailReceivedTopic, uuid.NewString(),
		cfg.BatchSize, cfg.MaxAttempts, cfg.ClaimTTL,
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Email-received producer running")
	runner := outbox.NewRunner(outbox.WithPollInterval(cfg.PollInterval))
	if err := runner.Run(runCtx, adapter.PollOnce); err != nil {
		log.Info().Err(err).Msg("Email-received producer stopped")
	}
}
