package main

import (
	"context"
	"os/signal"
	"planeo/libs/eventschemas"
	"planeo/libs/inbox"
	"planeo/libs/kafka"
	"planeo/libs/logger"
	"planeo/libs/schemaregistry"
	"planeo/services/core/internal/domain/category"
	domaininbox "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/domain/request"
	coreinbox "planeo/services/core/internal/infra/inbox"
	"planeo/services/core/internal/infra/llm"
	"planeo/services/core/internal/infra/postgres"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	log := logger.New("email-received-consumer")
	ctx := logger.WithContext(context.Background(), log)

	log.Info().Msg("Loading environment variables")
	cfg := LoadConfig(ctx)

	db := postgres.NewClient(ctx, cfg.DatabaseConfig())
	defer db.Close()

	categoryService := category.NewService(db)
	requestService := request.NewService(db)

	llmClient, err := llm.NewClient()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create LLM client")
	}

	registryClient, err := schemaregistry.NewClient(schemaregistry.Config{URL: cfg.SchemaRegistryURL})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create schema registry client")
	}
	serde := schemaregistry.NewSerde(registryClient)
	if err := schemaregistry.RegisterDecode[eventschemas.EmailReceived](
		ctx, serde, eventschemas.EmailReceivedSubject, eventschemas.EmailReceivedSchema,
	); err != nil {
		log.Fatal().Err(err).Msg("Failed to register EmailReceived for decoding")
	}

	instanceID := uuid.NewString()

	inboxService := domaininbox.NewService(db, requestService, categoryService, llmClient)
	emailReceivedConsumer := coreinbox.NewEmailReceivedConsumer(
		inboxService, serde, eventschemas.EmailReceivedTopic, instanceID,
		cfg.BatchSize, cfg.MaxAttempts, cfg.ClaimTTL,
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	kafkaClient, err := kafka.NewClient(
		kafka.Config{
			Brokers:    brokers,
			Username:   cfg.KafkaSaslUsername,
			Password:   cfg.KafkaSaslPassword,
			Mechanism:  cfg.KafkaSaslMechanism,
			TLSEnabled: cfg.KafkaTlsEnabled,
		},
		kgo.ConsumerGroup(cfg.GroupName),
		kgo.ConsumeTopics(eventschemas.EmailReceivedTopic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Kafka")
	}

	consumer := inbox.NewConsumer(kafkaClient, eventschemas.EmailReceivedTopic, inboxService)
	if err := consumer.Run(runCtx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start email-received consumer")
	}

	log.Info().Msg("Email-received consumer running")
	runner := inbox.NewRunner(inbox.WithPollInterval(cfg.PollInterval))
	if err := runner.Run(runCtx, emailReceivedConsumer.PollOnce); err != nil {
		log.Info().Err(err).Msg("Email-received consumer stopped")
	}
}
