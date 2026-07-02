// services/email/internal/infra/email/imap_service.go
package email

import (
	"context"
	"fmt"
	"io"
	"log"
	appError "planeo/libs/errors"
	"planeo/libs/logger"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
)

type IMAPSettings struct {
	Host     string
	Port     int
	Username string
	Password string
}

type Email struct {
	Subject   string
	Body      string
	From      string
	Date      time.Time
	MessageID string
	SeqNum    uint32
}

type IMAPService struct{}

func NewIMAPService() *IMAPService {
	return &IMAPService{}
}

func (s *IMAPService) TestConnection(ctx context.Context, settings IMAPSettings) error {
	c, err := s.login(ctx, settings)
	if err != nil {
		return err
	}
	defer c.Logout()
	return nil
}

func (s *IMAPService) MarkMailsAsUnseen(ctx context.Context, settings IMAPSettings, emails []Email) error {
	l := logger.FromContext(ctx)
	c, err := s.login(ctx, settings)
	if err != nil {
		return err
	}
	defer c.Logout()

	seqSet := imap.SeqSet{}
	for _, email := range emails {
		seqSet.AddNum(email.SeqNum)
	}

	storeFlags := imap.StoreFlags{Op: imap.StoreFlagsDel, Flags: []imap.Flag{imap.FlagSeen}, Silent: true}
	if err := c.Store(seqSet, &storeFlags, nil).Close(); err != nil {
		l.Error().Err(err).Msg("failed to mark mails as unseen")
		return err
	}
	return nil
}

func (s *IMAPService) FetchAllUnseenMails(ctx context.Context, settings IMAPSettings) ([]Email, error) {
	l := logger.FromContext(ctx)
	c, err := s.login(ctx, settings)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	sc := imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}
	e, err := c.Search(&sc, nil).Wait()
	if err != nil {
		l.Error().Err(err).Msg("failed to search for unseen messages")
		return nil, err
	}

	emails := []Email{}
	if len(e.AllSeqNums()) == 0 {
		return emails, nil
	}

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
		email, err := s.extractMailData(ctx, msg)
		if err != nil {
			l.Error().Err(err).Msg("failed to extract mail data")
			return nil, err
		}
		emails = append(emails, email)
	}

	if err := fetchCmd.Close(); err != nil {
		l.Error().Err(err).Msg("failed to close FETCH command")
		return nil, err
	}

	storeFlags := imap.StoreFlags{Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagSeen}, Silent: true}
	if err := c.Store(seqSet, &storeFlags, nil).Close(); err != nil {
		l.Error().Err(err).Msg("failed to mark fetched mails as seen")
		return emails, err
	}

	return emails, nil
}

func (s *IMAPService) login(ctx context.Context, settings IMAPSettings) (*imapclient.Client, error) {
	l := logger.FromContext(ctx)
	address := fmt.Sprintf("%s:%d", settings.Host, settings.Port)
	var (
		c   *imapclient.Client
		err error
	)

	switch settings.Port {
	case 143:
		c, err = imapclient.DialStartTLS(address, nil)
	case 993:
		c, err = imapclient.DialTLS(address, nil)
	case 3143:
		c, err = imapclient.DialInsecure(address, nil)
	default:
		e := appError.New(appError.ValidationError, fmt.Sprintf("Invalid port: %v", settings.Port))
		l.Error().Err(e).Msg("can not connect to server")
		return nil, e
	}

	if err != nil {
		l.Error().Err(err).Msg("failed to dial IMAP server")
		return nil, err
	}

	if err := c.Login(settings.Username, settings.Password).Wait(); err != nil {
		l.Error().Err(err).Msg("failed to login")
		return nil, err
	}

	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		l.Error().Err(err).Msg("failed to select INBOX")
		return nil, err
	}

	return c, nil
}

func (s *IMAPService) extractMailData(ctx context.Context, msg *imapclient.FetchMessageData) (Email, error) {
	l := logger.FromContext(ctx)
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
		l.Error().Err(err).Msg("failed to create mail reader")
		return Email{}, err
	}

	email, err := s.extractHeaderFields(ctx, mr.Header, Email{})
	if err != nil {
		l.Error().Err(err).Msg("failed to extract header fields")
		return Email{}, err
	}

	email, err = s.extractEmailBody(ctx, mr, email)
	if err != nil {
		l.Error().Err(err).Msg("failed to extract email body")
		return Email{}, err
	}

	email.SeqNum = msg.SeqNum
	return email, nil
}

func (s *IMAPService) extractHeaderFields(ctx context.Context, h mail.Header, email Email) (Email, error) {
	l := logger.FromContext(ctx)

	date, err := h.Date()
	if err != nil {
		l.Error().Err(err).Msg("failed to parse Date header field")
		return Email{}, err
	}
	email.Date = date

	from, err := h.AddressList("From")
	if err != nil {
		l.Error().Err(err).Msg("failed to parse From header field")
		return Email{}, err
	}
	email.From = from[0].Address

	subject, err := h.Text("Subject")
	if err != nil {
		l.Error().Err(err).Msg("failed to parse Subject header field")
		return Email{}, err
	}
	email.Subject = subject

	messageId, err := h.MessageID()
	if err != nil {
		l.Error().Err(err).Msg("failed to parse Message-ID header field")
		return Email{}, err
	}
	email.MessageID = strings.Split(messageId, "@")[0]

	return email, nil
}

func (s *IMAPService) extractEmailBody(ctx context.Context, mr *mail.Reader, email Email) (Email, error) {
	l := logger.FromContext(ctx)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			l.Error().Err(err).Msg("failed to read message part")
			return Email{}, err
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			b, err := io.ReadAll(p.Body)
			if err != nil {
				l.Error().Err(err).Msg("failed to read message body")
				return Email{}, err
			}
			email.Body = string(b)
		case *mail.AttachmentHeader:
			_, _ = h.Filename()
		}
	}
	return email, nil
}
