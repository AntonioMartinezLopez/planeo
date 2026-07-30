package main

import (
	"context"
	"os/signal"
	domainoutbox "planeo/services/email/internal/domain/outbox"
	emailoutbox "planeo/services/email/internal/infra/outbox"
	"planeo/services/email/internal/infra/postgres"
	"strings"
	"syscall"

	"planeo/libs/logger"

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

	outboxService := domainoutbox.NewService(db)
	producer, err := emailoutbox.NewEmailReceivedProducer(ctx, outboxService, emailoutbox.Config{
		KafkaBrokers:    strings.Split(cfg.KafkaBrokers, ","),
		KafkaUsername:   cfg.KafkaSaslUsername,
		KafkaPassword:   cfg.KafkaSaslPassword,
		KafkaMechanism:  cfg.KafkaSaslMechanism,
		KafkaTLSEnabled: cfg.KafkaTlsEnabled,
		RegistryURL:     cfg.SchemaRegistryURL,
		InstanceID:      uuid.NewString(),
		BatchSize:       cfg.BatchSize, MaxAttempts: cfg.MaxAttempts,
		ClaimTTL: cfg.ClaimTTL, PollInterval: cfg.PollInterval,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create email-received producer")
	}
	defer producer.Close()

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Email-received producer running")
	if err := producer.Run(runCtx); err != nil {
		log.Info().Err(err).Msg("Email-received producer stopped")
	}
}
