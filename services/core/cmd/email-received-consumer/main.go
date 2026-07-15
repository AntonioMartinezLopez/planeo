package main

import (
	"context"
	"os/signal"
	"planeo/libs/events/contracts"
	"planeo/libs/inbox"
	"planeo/libs/logger"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
	coreinbox "planeo/services/core/internal/infra/inbox"
	"planeo/services/core/internal/infra/postgres"
	"strings"
	"syscall"

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

	instanceID := uuid.NewString()

	adapter := coreinbox.NewEmailReceivedConsumerAdapter(
		db, requestService, categoryService, contracts.EmailReceivedTopic, instanceID,
		cfg.BatchSize, cfg.MaxAttempts, cfg.ClaimTTL,
	)

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	consumer := inbox.NewConsumer(brokers, cfg.GroupName, contracts.EmailReceivedTopic, db)
	if err := consumer.Run(runCtx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start email-received consumer")
	}

	log.Info().Msg("Email-received consumer running")
	runner := inbox.NewRunner(inbox.WithPollInterval(cfg.PollInterval))
	if err := runner.Run(runCtx, adapter.PollOnce); err != nil {
		log.Info().Err(err).Msg("Email-received consumer stopped")
	}
}
