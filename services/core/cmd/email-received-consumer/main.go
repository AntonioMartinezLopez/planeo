package main

import (
	"context"
	"os/signal"
	"planeo/libs/events/contracts"
	"planeo/libs/inbox"
	"planeo/libs/logger"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
	coreEvents "planeo/services/core/internal/infra/events"
	"planeo/services/core/internal/infra/postgres"
	"strings"
	"syscall"
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

	handler := coreEvents.CreateInboxHandler(coreEvents.Services{
		RequestService:  requestService,
		CategoryService: categoryService,
	})

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	consumer := inbox.NewConsumer(brokers, cfg.GroupName, contracts.EmailReceivedTopic, db)
	if err := consumer.Run(runCtx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start email-received consumer")
	}

	worker := inbox.NewWorker(db, handler,
		inbox.WithPollInterval(cfg.PollInterval),
		inbox.WithBatchSize(cfg.BatchSize),
		inbox.WithMaxAttempts(cfg.MaxAttempts),
		inbox.WithClaimTTL(cfg.ClaimTTL),
	)

	log.Info().Msg("Email-received consumer running")
	if err := worker.Run(runCtx); err != nil {
		log.Info().Err(err).Msg("Email-received consumer stopped")
	}
}
