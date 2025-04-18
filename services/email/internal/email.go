package internal

import (
	"context"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/email/internal/resources/settings/models"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

type CronServiceInterface interface {
	AddJob(ctx context.Context, task func(), fetchInterval time.Duration, tags []string)
	RemoveJobByTag(ctx context.Context, tag string)
}

type IMAPServiceInterface interface {
	FetchAllUnseenMails(context context.Context, settings IMAPSettings) ([]Email, error)
	TestConnection(ctx context.Context, settings IMAPSettings) error
	MarkMailsAsUnseen(ctx context.Context, settings IMAPSettings, emails []Email) error
}

type EventServiceInterface interface {
	PublishEmailReceived(ctx context.Context, payload events.EmailCreatedPayload) error
	IsConnected() bool
}

type EmailService struct {
	cronService  CronServiceInterface
	imapService  IMAPServiceInterface
	eventService EventServiceInterface
	logger       zerolog.Logger
}

func NewEmailService(cronService CronServiceInterface, imapService IMAPServiceInterface, eventService EventServiceInterface) *EmailService {
	return &EmailService{
		cronService:  cronService,
		imapService:  imapService,
		eventService: eventService,
		logger:       logger.New("email-service"),
	}
}

func (s *EmailService) StartFetching(ctx context.Context, settings []models.Setting) error {

	for _, setting := range settings {

		task := s.createTask(setting)
		s.cronService.AddJob(ctx, task, time.Second*10, []string{strconv.Itoa(setting.ID)})

	}
	return nil
}

func (s *EmailService) StopFetching(ctx context.Context, settingsId int) {
	s.cronService.RemoveJobByTag(ctx, strconv.Itoa(settingsId))
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

		imapSettings := IMAPSettings{
			Host:     settings.Host,
			Port:     settings.Port,
			Username: settings.Username,
			Password: settings.Password,
		}

		mails, err := s.imapService.FetchAllUnseenMails(ctx, imapSettings)

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

		for _, mail := range mails {

			emailLogger.Info().
				Str("message_id", mail.MessageID).
				Int("organization_id", settings.OrganizationID).
				Msg("Processing email")

			err = s.eventService.PublishEmailReceived(ctx, events.EmailCreatedPayload{
				Subject:        mail.Subject,
				Body:           mail.Body,
				From:           mail.From,
				Date:           mail.Date,
				MessageID:      mail.MessageID,
				OrganizationId: settings.OrganizationID,
			})

			if err != nil {
				emailLogger.Error().
					Err(err).
					Int("organization_id", settings.OrganizationID).
					Str("message_id", mail.MessageID).
					Msg("Error publishing email received event")
			}
		}

	}
}
