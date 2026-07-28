package main

import (
	"context"
	"os/signal"
	"planeo/libs/eventschemas"
	"planeo/libs/kafka"
	"planeo/libs/logger"
	"planeo/libs/outbox"
	"planeo/libs/schemaregistry"
	domainoutbox "planeo/services/email/internal/domain/outbox"
	"planeo/services/email/internal/infra/events"
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
	kafkaClient, err := kafka.NewClient(kafka.Config{
		Brokers:    brokers,
		Username:   cfg.KafkaSaslUsername,
		Password:   cfg.KafkaSaslPassword,
		Mechanism:  cfg.KafkaSaslMechanism,
		TLSEnabled: cfg.KafkaTlsEnabled,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}
	defer kafkaClient.Close()

	registryClient, err := schemaregistry.NewClient(schemaregistry.Config{URL: cfg.SchemaRegistryURL})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create schema registry client")
	}
	serde := schemaregistry.NewSerde(registryClient)
	if err := schemaregistry.RegisterEncode[eventschemas.EmailReceived](
		ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailReceivedSchema,
	); err != nil {
		log.Fatal().Err(err).Msg("Failed to register EmailReceived for encoding")
	}

	rawProducer := outbox.NewProducer(kafkaClient)
	producer := events.NewEmailReceivedProducer(rawProducer, serde)

	outboxService := domainoutbox.NewService(db, producer)
	emailReceivedProducer := emailoutbox.NewEmailReceivedProducer(
		outboxService, eventschemas.EmailReceivedTopic, uuid.NewString(),
		cfg.BatchSize, cfg.MaxAttempts, cfg.ClaimTTL,
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Email-received producer running")
	runner := outbox.NewRunner(outbox.WithPollInterval(cfg.PollInterval))
	if err := runner.Run(runCtx, emailReceivedProducer.PollOnce); err != nil {
		log.Info().Err(err).Msg("Email-received producer stopped")
	}
}
