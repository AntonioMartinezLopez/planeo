package email

import (
	"context"
	"encoding/json"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/email/internal/domain/mail"
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
	FetchUnseenMails(ctx context.Context, settings IMAPSettings) ([]Email, error)
	MarkSeen(ctx context.Context, settings IMAPSettings, uids []uint32) error
	TestConnection(ctx context.Context, settings IMAPSettings) error
}

type mailServiceInterface interface {
	SaveFetchedMails(ctx context.Context, mails []mail.FetchedMail) ([]mail.SaveResult, error)
}

type EmailService struct {
	cronService cronServiceInterface
	imapService imapServiceInterface
	mailService mailServiceInterface
	logger      zerolog.Logger
}

func NewEmailService(cron cronServiceInterface, imap imapServiceInterface, mailService mailServiceInterface) *EmailService {
	return &EmailService{
		cronService: cron,
		imapService: imap,
		mailService: mailService,
		logger:      logger.New("email-service"),
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

		mails, err := s.imapService.FetchUnseenMails(ctx, imapSettings)
		duration := time.Since(start)

		if err != nil {
			emailLogger.Error().Err(err).Dur("duration_ms", duration).Msg("Error fetching emails")
			return
		}

		emailLogger.Info().
			Int("email_count", len(mails)).
			Dur("duration_ms", duration).
			Msg("Email fetch completed")

		if len(mails) == 0 {
			return
		}

		fetched := make([]mail.FetchedMail, 0, len(mails))
		for _, m := range mails {
			payload, err := json.Marshal(events.EmailCreatedPayload{
				Subject:        m.Subject,
				Body:           m.Body,
				From:           m.From,
				Date:           m.Date,
				MessageID:      m.MessageID,
				OrganizationId: st.OrganizationID,
			})
			if err != nil {
				emailLogger.Error().Err(err).Str("message_id", m.MessageID).Msg("Error marshaling email event payload")
				continue
			}

			fetched = append(fetched, mail.FetchedMail{
				Mail: mail.NewMail{
					MessageID:      m.MessageID,
					SettingID:      st.ID,
					OrganizationID: st.OrganizationID,
					Subject:        m.Subject,
					Sender:         m.From,
					Body:           m.Body,
					Date:           m.Date,
				},
				Event: mail.OutboxEvent{
					Topic:   events.EmailReceivedTopic,
					Key:     []byte(strconv.Itoa(st.OrganizationID)),
					Payload: payload,
				},
				UID: m.UID,
			})
		}

		if len(fetched) == 0 {
			return
		}

		results, err := s.mailService.SaveFetchedMails(ctx, fetched)
		if err != nil {
			emailLogger.Error().Err(err).Msg("Error saving fetched mails to outbox")
			return
		}

		uids := make([]uint32, 0, len(results))
		for _, r := range results {
			uids = append(uids, r.UID)
		}

		if err := s.imapService.MarkSeen(ctx, imapSettings, uids); err != nil {
			emailLogger.Error().Err(err).Msg("Error marking emails as seen")
		}
	}
}
