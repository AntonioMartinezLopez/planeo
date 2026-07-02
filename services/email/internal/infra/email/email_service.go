package email

import (
	"context"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/email/internal/domain/setting"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

type cronServiceInterface interface {
	AddJob(ctx context.Context, task func(), fetchInterval time.Duration, tags []string)
	RemoveJobByTag(ctx context.Context, tag string)
}

type imapServiceInterface interface {
	FetchAllUnseenMails(ctx context.Context, settings IMAPSettings) ([]Email, error)
	TestConnection(ctx context.Context, settings IMAPSettings) error
	MarkMailsAsUnseen(ctx context.Context, settings IMAPSettings, emails []Email) error
}

type eventServiceInterface interface {
	PublishEmailReceived(ctx context.Context, payload events.EmailCreatedPayload) error
}

type EmailService struct {
	cronService  cronServiceInterface
	imapService  imapServiceInterface
	eventService eventServiceInterface
	logger       zerolog.Logger
}

func NewEmailService(cron cronServiceInterface, imap imapServiceInterface, eventService eventServiceInterface) *EmailService {
	return &EmailService{
		cronService:  cron,
		imapService:  imap,
		eventService: eventService,
		logger:       logger.New("email-service"),
	}
}

func (s *EmailService) StartFetching(ctx context.Context, settings []setting.Setting) error {
	for _, st := range settings {
		task := s.createTask(st)
		s.cronService.AddJob(ctx, task, 10*time.Second, []string{strconv.Itoa(st.ID)})
	}
	return nil
}

func (s *EmailService) StopFetching(ctx context.Context, settingId int) {
	s.cronService.RemoveJobByTag(ctx, strconv.Itoa(settingId))
}

func (s *EmailService) TestConnection(ctx context.Context, st setting.Setting) error {
	return s.imapService.TestConnection(ctx, IMAPSettings{
		Host:     st.Host,
		Port:     st.Port,
		Username: st.Username,
		Password: st.Password,
	})
}

func (s *EmailService) createTask(st setting.Setting) func() {
	return func() {
		start := time.Now()
		emailLogger := s.logger.With().Int("setting_id", st.ID).Logger()
		ctx := logger.WithContext(context.Background(), emailLogger)

		imapSettings := IMAPSettings{
			Host:     st.Host,
			Port:     st.Port,
			Username: st.Username,
			Password: st.Password,
		}

		mails, err := s.imapService.FetchAllUnseenMails(ctx, imapSettings)
		duration := time.Since(start)

		if err != nil {
			emailLogger.Error().Err(err).Dur("duration_ms", duration).Msg("Error fetching emails")
		}

		emailLogger.Info().
			Int("email_count", len(mails)).
			Dur("duration_ms", duration).
			Msg("Email fetch completed")

		for _, mail := range mails {
			emailLogger.Info().
				Str("message_id", mail.MessageID).
				Int("organization_id", st.OrganizationID).
				Msg("Processing email")

			if err := s.eventService.PublishEmailReceived(ctx, events.EmailCreatedPayload{
				Subject:        mail.Subject,
				Body:           mail.Body,
				From:           mail.From,
				Date:           mail.Date,
				MessageID:      mail.MessageID,
				OrganizationId: st.OrganizationID,
			}); err != nil {
				emailLogger.Error().
					Err(err).
					Int("organization_id", st.OrganizationID).
					Str("message_id", mail.MessageID).
					Msg("Error publishing email received event")
			}
		}
	}
}
