package main

import (
	"context"
	"os/signal"
	"planeo/libs/eventschemas"
	libsinbox "planeo/libs/inbox"
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
	decoder := schemaregistry.NewDecoder[eventschemas.EmailReceived](serde, eventschemas.EmailReceivedSchema)

	inboxService := domaininbox.NewService(db, requestService, categoryService, llmClient)

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

	rawConsumer := libsinbox.NewConsumer(kafkaClient, eventschemas.EmailReceivedTopic, inboxService)

	emailReceivedConsumer := coreinbox.NewEmailReceivedConsumer(
		coreinbox.Deps{
			KafkaConsumer: rawConsumer,
			Service:       inboxService,
			Decoder:       decoder,
		},
		coreinbox.Config{
			Topic:        eventschemas.EmailReceivedTopic,
			InstanceID:   uuid.NewString(),
			BatchSize:    cfg.BatchSize,
			MaxAttempts:  cfg.MaxAttempts,
			ClaimTTL:     cfg.ClaimTTL,
			PollInterval: cfg.PollInterval,
		},
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Email-received consumer running")
	if err := emailReceivedConsumer.Run(runCtx); err != nil {
		log.Info().Err(err).Msg("Email-received consumer stopped")
	}
}
