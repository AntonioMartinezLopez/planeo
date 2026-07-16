package email

import (
	"context"
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
	SaveFetchedMails(ctx context.Context, mails []mail.RawFetchedMail) ([]mail.SaveResult, error)
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

		raws := make([]mail.RawFetchedMail, 0, len(mails))
		for _, m := range mails {
			raws = append(raws, mail.RawFetchedMail{
				MessageID:      m.MessageID,
				SettingID:      st.ID,
				OrganizationID: st.OrganizationID,
				Subject:        m.Subject,
				Sender:         m.From,
				Body:           m.Body,
				Date:           m.Date,
				UID:            m.UID,
			})
		}

		emailLogger.Info().Int("batch_size", len(raws)).Msg("Saving fetched mails to outbox")

		results, err := s.mailService.SaveFetchedMails(ctx, raws)
		if err != nil {
			emailLogger.Error().Err(err).Int("batch_size", len(raws)).Msg("Error saving fetched mails to outbox")
			return
		}

		inserted := 0
		uids := make([]uint32, 0, len(results))
		for _, r := range results {
			uids = append(uids, r.UID)
			if r.Inserted {
				inserted++
			}
		}

		emailLogger.Info().
			Int("results_count", len(results)).
			Int("inserted_count", inserted).
			Msg("SaveFetchedMails completed")

		if err := s.imapService.MarkSeen(ctx, imapSettings, uids); err != nil {
			emailLogger.Error().Err(err).Msg("Error marking emails as seen")
			return
		}

		emailLogger.Info().Int("marked_seen_count", len(uids)).Msg("Marked mails as seen on IMAP")
	}
}
