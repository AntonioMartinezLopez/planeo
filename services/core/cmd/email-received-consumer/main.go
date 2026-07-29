package main

import (
	"context"
	"os/signal"
	"planeo/services/core/internal/domain/category"
	domaininbox "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/domain/request"
	coreinbox "planeo/services/core/internal/infra/inbox"
	"planeo/services/core/internal/infra/llm"
	"planeo/services/core/internal/infra/postgres"
	"strings"
	"syscall"

	"planeo/libs/logger"

	"github.com/google/uuid"
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

	inboxService := domaininbox.NewService(db, requestService, categoryService, llmClient)

	consumer, err := coreinbox.NewEmailReceivedConsumer(ctx, inboxService, coreinbox.Config{
		KafkaBrokers:    strings.Split(cfg.KafkaBrokers, ","),
		KafkaUsername:   cfg.KafkaSaslUsername,
		KafkaPassword:   cfg.KafkaSaslPassword,
		KafkaMechanism:  cfg.KafkaSaslMechanism,
		KafkaTLSEnabled: cfg.KafkaTlsEnabled,
		GroupName:       cfg.GroupName,
		RegistryURL:     cfg.SchemaRegistryURL,
		InstanceID:      uuid.NewString(),
		BatchSize:       cfg.BatchSize, MaxAttempts: cfg.MaxAttempts,
		ClaimTTL: cfg.ClaimTTL, PollInterval: cfg.PollInterval,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create email-received consumer")
	}

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("Email-received consumer running")
	if err := consumer.Run(runCtx); err != nil {
		log.Info().Err(err).Msg("Email-received consumer stopped")
	}
}
