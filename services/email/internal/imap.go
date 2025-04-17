package internal

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

type IMAPService struct {
}

func NewIMAPService() *IMAPService {
	return &IMAPService{}
}

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

func (s *IMAPService) TestConnection(ctx context.Context, settings IMAPSettings) error {
	c, err := s.login(ctx, settings)
	defer c.Logout()

	return err
}

func (s *IMAPService) MarkMailsAsUnseen(ctx context.Context, settings IMAPSettings, emails []Email) error {
	logger := logger.FromContext(ctx)
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
		logger.Error().Err(err).Msgf("failed to mark mails as unseen")
		return err
	}

	return nil
}

func (s *IMAPService) FetchAllUnseenMails(ctx context.Context, settings IMAPSettings) ([]Email, error) {

	logger := logger.FromContext(ctx)
	c, err := s.login(ctx, settings)

	if err != nil {
		return nil, err
	}
	defer c.Logout()

	// Fetch all unseen messages from inbox
	sc := imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}
	e, err := c.Search(&sc, nil).Wait()

	if err != nil {
		logger.Error().Err(err).Msgf("failed to search for unseen messages")
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

			email, err := s.extractMailData(ctx, msg)
			if err != nil {
				logger.Error().Err(err).Msgf("failed to extract mail data")
				return nil, err
			}
			emails = append(emails, email)
		}

		if err := fetchCmd.Close(); err != nil {
			logger.Error().Err(err).Msgf("failed to close FETCH command")
			return nil, err
		}

		// mark fetched mails as seen
		storeFlags := imap.StoreFlags{Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagSeen}, Silent: true}
		if err := c.Store(seqSet, &storeFlags, nil).Close(); err != nil {
			logger.Error().Err(err).Msgf("failed to mark fetched mails as seen")
			return emails, err
		}

	}
	return emails, nil
}

func (s *IMAPService) login(ctx context.Context, settings IMAPSettings) (*imapclient.Client, error) {
	logger := logger.FromContext(ctx)
	imapClient := new(imapclient.Client)
	address := fmt.Sprintf("%s:%d", settings.Host, settings.Port)
	var err error

	// connect to the IMAP server and login
	switch settings.Port {
	case 143:
		imapClient, err = imapclient.DialStartTLS(address, nil)
	case 993:
		imapClient, err = imapclient.DialTLS(address, nil)
	case 3143:
		imapClient, err = imapclient.DialInsecure(address, nil)
	default:
		err := appError.New(appError.ValidationError, fmt.Sprintf("Invalid port: %v", settings.Port))
		logger.Error().Err(err).Msgf("can not connect to server")
		return nil, err
	}

	if err != nil {
		logger.Error().Err(err).Msgf("failed to dial IMAP server")
		return nil, err
	}

	if err := imapClient.Login(settings.Username, settings.Password).Wait(); err != nil {
		logger.Error().Err(err).Msgf("failed to login")
		return nil, err
	}

	// check that INBOX exists
	_, err = imapClient.Select("INBOX", nil).Wait()
	if err != nil {
		logger.Error().Err(err).Msgf("failed to select INBOX")
		return nil, err
	}

	return imapClient, nil
}

func (s *IMAPService) extractMailData(ctx context.Context, msg *imapclient.FetchMessageData) (Email, error) {

	logger := logger.FromContext(ctx)
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
		logger.Error().Err(err).Msgf("failed to create mail reader")
		return Email{}, err
	}

	// Extract header fields
	email, err = s.extractHeaderFields(ctx, mr.Header, email)
	if err != nil {
		logger.Error().Err(err).Msgf("failed to extract header fields")
		return Email{}, err
	}

	// Process the message's parts
	email, err = s.extractEmailBody(ctx, mr, email)
	if err != nil {
		logger.Error().Err(err).Msgf("failed to extract email body")
		return Email{}, err
	}

	// Set the sequence number
	email.SeqNum = msg.SeqNum

	return email, nil
}

func (s *IMAPService) extractHeaderFields(ctx context.Context, h mail.Header, email Email) (Email, error) {
	logger := logger.FromContext(ctx)

	if date, err := h.Date(); err != nil {
		logger.Error().Err(err).Msgf("failed to parse Date header field")
		return Email{}, err
	} else {
		email.Date = date
	}

	if from, err := h.AddressList("From"); err != nil {
		logger.Error().Err(err).Msgf("failed to parse From header field")
		return Email{}, err
	} else {
		email.From = from[0].Address
	}

	if subject, err := h.Text("Subject"); err != nil {
		logger.Error().Err(err).Msgf("failed to parse Subject header field")
		return Email{}, err
	} else {
		email.Subject = subject
	}
	if messageId, err := h.MessageID(); err != nil {
		logger.Error().Err(err).Msgf("failed to parse Message-ID header field")
		return Email{}, err
	} else {
		email.MessageID = strings.Split(messageId, "@")[0]
	}

	return email, nil
}

func (s *IMAPService) extractEmailBody(context context.Context, mr *mail.Reader, email Email) (Email, error) {
	logger := logger.FromContext(context)

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			logger.Error().Err(err).Msgf("failed to read message part")
			return Email{}, err
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			b, err := io.ReadAll(p.Body)
			if err != nil {
				logger.Error().Err(err).Msgf("failed to read message body")
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
