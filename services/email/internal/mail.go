package internal

import (
	"io"
	"log"
	"planeo/libs/logger"
	"planeo/services/email/internal/resources/settings/models"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type CronServiceInterface interface {
	AddJob(task func(), fetchInterval time.Duration, tags []string)
	GetJob(id uuid.UUID) *gocron.Job
	GetJobByTag(tag string) *gocron.Job
	RemoveJob(id uuid.UUID) error
	RemoveJobByTag(tag string)
}

type EmailService struct {
	cronService CronServiceInterface
	logger      zerolog.Logger
}

type Email struct {
	Subject   string
	Body      string
	From      string
	Date      time.Time
	MessageID string
}

func NewEmailService(cronService CronServiceInterface) *EmailService {
	return &EmailService{
		cronService: cronService,
		logger:      logger.New("email-service"),
	}
}

func (s *EmailService) StartFetching(settings []models.Setting) error {

	for _, setting := range settings {

		task := s.createTask(setting)
		s.cronService.AddJob(task, time.Second*10, []string{strconv.Itoa(setting.ID)})

	}
	return nil
}

func (s *EmailService) StopFetching() error {
	return nil
}

func (s *EmailService) GetFetchingStatus(_ models.Setting) error {
	return nil
}

func (s *EmailService) TestConnection(_ models.Setting) error {
	return nil
}

func (s *EmailService) createTask(settings models.Setting) func() {
	return func() {
		mails, err := s.fetchMails(settings)
		if err != nil {
			s.logger.Error().Err(err).Msgf("Error fetching emails")
		}
		s.logger.Info().Int("setting_id", settings.ID).Msgf("Fetched %d emails.", len(mails))
	}
}

func (s *EmailService) fetchMails(settings models.Setting) ([]Email, error) {
	var c *imapclient.Client
	var err error

	// connect to the IMAP server and login
	switch settings.Port {
	case 143:
		c, err = imapclient.DialStartTLS(settings.Host, nil)
	case 993:
		c, err = imapclient.DialTLS(settings.Host, nil)
	case 3143:
		c, err = imapclient.DialInsecure("localhost:3143", nil)
	default:
		s.logger.Error().Int("setting_id", settings.ID).Msgf("Invalid port: %v", settings.Port)
		return nil, nil
	}

	if err != nil {
		s.logger.Error().Err(err).Msgf("failed to dial IMAP server")
		return nil, err
	}
	defer c.Close()

	if err := c.Login(settings.Username, settings.Password).Wait(); err != nil {
		s.logger.Error().Err(err).Msgf("failed to login")
		return nil, err
	}

	// check that INBOX exists
	_, err = c.Select("INBOX", nil).Wait()
	if err != nil {
		s.logger.Error().Err(err).Msgf("failed to select INBOX")
		return nil, err
	}

	// Fetch all unseen messages from inbox
	sc := imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}
	e, err := c.Search(&sc, nil).Wait()

	if err != nil {
		s.logger.Error().Err(err).Msgf("failed to search for unseen messages")
		return nil, err
	}

	emails := []Email{}

	if len(e.AllSeqNums()) > 0 {

		seqSet := imap.SeqSet{}
		seqSet.AddNum(e.AllSeqNums()...)
		fetchOptions := &imap.FetchOptions{
			BodySection: []*imap.FetchItemBodySection{{}},
		}
		fetchCmd := c.Fetch(seqSet, fetchOptions)
		defer fetchCmd.Close()

		for {
			msg := fetchCmd.Next()
			if msg == nil {
				break
			}

			email, err := s.extractMailData(msg)
			if err != nil {
				s.logger.Error().Err(err).Msgf("failed to extract mail data")
				return nil, err
			}
			emails = append(emails, email)
		}

		if err := fetchCmd.Close(); err != nil {
			s.logger.Error().Err(err).Msgf("failed to close FETCH command")
			return nil, err
		}

		// mark fetched mails as seen
		storeFlags := imap.StoreFlags{Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagSeen}, Silent: true}
		if err := c.Store(seqSet, &storeFlags, nil).Close(); err != nil {
			s.logger.Error().Err(err).Msgf("failed to mark fetched mails as seen")
			return emails, err
		}

	}

	if err := c.Logout().Wait(); err != nil {
		s.logger.Error().Err(err).Msgf("failed to logout")
		return emails, err
	}

	return emails, nil
}

func (s *EmailService) extractMailData(msg *imapclient.FetchMessageData) (Email, error) {

	email := Email{}

	// Find the body section in the response
	var bodySection imapclient.FetchItemDataBodySection
	ok := false
	for {
		item := msg.Next()
		if item == nil {
			break
		}
		bodySection, ok = item.(imapclient.FetchItemDataBodySection)
		if ok {
			break
		}
	}
	if !ok {
		log.Fatalf("FETCH command did not return body section")
	}

	mr, err := mail.CreateReader(bodySection.Literal)
	if err != nil {
		s.logger.Error().Err(err).Msgf("failed to create mail reader")
		return Email{}, err
	}

	// Extract header fields
	email, err = s.extractHeaderFields(mr.Header, email)
	if err != nil {
		s.logger.Error().Err(err).Msgf("failed to extract header fields")
		return Email{}, err
	}

	// Process the message's parts
	email, err = s.extractEmailBody(mr, email)
	if err != nil {
		s.logger.Error().Err(err).Msgf("failed to extract email body")
		return Email{}, err
	}

	return email, nil
}

func (s *EmailService) extractHeaderFields(h mail.Header, email Email) (Email, error) {
	if date, err := h.Date(); err != nil {
		s.logger.Error().Err(err).Msgf("failed to parse Date header field")
		return Email{}, err
	} else {
		email.Date = date
	}

	if from, err := h.AddressList("From"); err != nil {
		s.logger.Error().Err(err).Msgf("failed to parse From header field")
		return Email{}, err
	} else {
		email.From = from[0].Address
	}

	if subject, err := h.Text("Subject"); err != nil {
		s.logger.Error().Err(err).Msgf("failed to parse Subject header field")
		return Email{}, err
	} else {
		email.Subject = subject
	}
	if messageId, err := h.MessageID(); err != nil {
		s.logger.Error().Err(err).Msgf("failed to parse Message-ID header field")
		return Email{}, err
	} else {
		email.MessageID = strings.Split(messageId, "@")[0]
	}

	return email, nil
}

func (s *EmailService) extractEmailBody(mr *mail.Reader, email Email) (Email, error) {
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			s.logger.Error().Err(err).Msgf("failed to read message part")
			return Email{}, err
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			b, err := io.ReadAll(p.Body)
			if err != nil {
				s.logger.Error().Err(err).Msgf("failed to read message body")
				return Email{}, err
			}
			email.Body = string(b)
		case *mail.AttachmentHeader:
			// This is an attachment
			_, _ = h.Filename()

			// TODO: Save the attachment in suitable location, e.g. S3
			// file, err := os.Create(filename)
			// if err != nil {
			// 	log.Fatal(err)
			// }
			// size, err := io.Copy(file, p.Body)
			// if err != nil {
			// 	log.Fatal(err)
			// }
			// log.Printf("Saved %v bytes into %v\n", size, filename)
		}
	}
	return email, nil
}
