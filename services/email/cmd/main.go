package main

import (
	"context"
	"net/http"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/email/internal/config"
	"planeo/services/email/internal/domain/setting"
	emailInfra "planeo/services/email/internal/infra/email"
	"planeo/services/email/internal/infra/postgres"
	"planeo/services/email/internal/infra/rest"
	"time"
)

func main() {
	logConfig := logger.DefaultConfig()
	logger.Setup(logConfig)
	log := logger.New("main")
	ctx := logger.WithContext(context.Background(), log)

	log.Info().Msg("Loading environment variables")
	cfg := config.LoadConfig(ctx)

	db := postgres.NewClient(ctx, cfg.DatabaseConfig())
	defer db.Close()

	eventService, err := events.NewEventService(cfg.NatsUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to NATS")
	}

	cronService := emailInfra.NewCronService()
	cronService.Start()

	imapService := emailInfra.NewIMAPService()
	emailService := emailInfra.NewEmailService(cronService, imapService, eventService)

	settingService, err := setting.NewService(db, emailService)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize setting service")
	}

	server := rest.New(cfg, rest.Services{SettingService: settingService})

	httpServer := http.Server{
		Addr:              cfg.ServerConfig(),
		Handler:           server.Router,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	log.Info().Msgf("Server Running at %s", cfg.ServerConfig())
	log.Fatal().Msgf("%v", httpServer.ListenAndServe())
}
