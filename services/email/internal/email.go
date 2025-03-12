package internal

import (
	"context"
	"planeo/libs/logger"
	"planeo/services/email/internal/resources/settings/models"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

type CronServiceInterface interface {
	AddJob(task func(), fetchInterval time.Duration, tags []string)
	RemoveJobByTag(tag string)
}

type IMAPServiceInterface interface {
	FetchAllUnseenMails(context context.Context, settings IMAPSettings) ([]Email, error)
	TestConnection(ctx context.Context, settings IMAPSettings) error
}

type EmailService struct {
	cronService CronServiceInterface
	imapService IMAPServiceInterface
	logger      zerolog.Logger
}

func NewEmailService(cronService CronServiceInterface, imapService IMAPServiceInterface) *EmailService {
	return &EmailService{
		cronService: cronService,
		imapService: imapService,
		logger:      logger.New("email-service"),
	}
}

func (s *EmailService) StartFetching(ctx context.Context, settings []models.Setting) error {

	for _, setting := range settings {

		task := s.createTask(setting)
		s.cronService.AddJob(task, time.Second*10, []string{strconv.Itoa(setting.ID)})

	}
	return nil
}

func (s *EmailService) StopFetching(ctx context.Context, settingsId int) {
	s.cronService.RemoveJobByTag(strconv.Itoa(settingsId))
}

func (s *EmailService) TestConnection(ctx context.Context, settings models.Setting) error {
	return s.imapService.TestConnection(ctx, IMAPSettings{
		Host:     settings.Host,
		Port:     settings.Port,
		Username: settings.Username,
		Password: settings.Password,
	})
}

func (s *EmailService) createTask(settings models.Setting) func() {
	return func() {
		start := time.Now()
		emailLogger := s.logger.With().Int("setting_id", settings.ID).Logger()
		ctx := logger.WithContext(context.Background(), emailLogger)

		mails, err := s.imapService.FetchAllUnseenMails(ctx, IMAPSettings{
			Host:     settings.Host,
			Port:     settings.Port,
			Username: settings.Username,
			Password: settings.Password,
		})

		duration := time.Since(start)

		if err != nil {
			emailLogger.Error().
				Err(err).
				Dur("duration_ms", duration).
				Msg("Error fetching emails")
		}

		emailLogger.Info().
			Int("email_count", len(mails)).
			Dur("duration_ms", duration).
			Msg("Email fetch completed")
	}
}
