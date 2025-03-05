package internal

import (
	"fmt"
	"log"
	"planeo/libs/logger"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func FetchMail() {

	logger.Info("Hello, World!")
	var c *imapclient.Client
	var err error

	port := 3143
	host := "localhost"
	address := fmt.Sprintf("%s:%d", host, port)

	// connect to the IMAP server and login
	switch port {
	case 143:
		c, err = imapclient.DialStartTLS(address, nil)
	case 993:
		c, err = imapclient.DialTLS(address, nil)
	case 3143:
		c, err = imapclient.DialInsecure("localhost:3143", nil)
	default:
		log.Fatalf("Invalid port: %v", port)
	}

	if err != nil {
		log.Fatalf("failed to dial IMAP server: %v", err)
	}
	defer c.Close()

	if err := c.Login("test@test.test", "test").Wait(); err != nil {
		log.Fatalf("failed to login: %v", err)
	}

	// check that INBOX exists
	mailboxes, err := c.List("", "%", nil).Collect()
	if err != nil {
		log.Fatalf("failed to list mailboxes: %v", err)
	}
	log.Printf("Found %v mailboxes", len(mailboxes))
	for _, mbox := range mailboxes {
		log.Printf(" - %v", mbox.Mailbox)
	}

	selectedMbox, err := c.Select("INBOX", nil).Wait()
	if err != nil {
		log.Fatalf("failed to select INBOX: %v", err)
	}
	log.Printf("INBOX contains %v messages", selectedMbox.NumMessages)

	// Fetch all unseen messages from inbox
	sc := imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}
	e, err := c.Search(&sc, nil).Wait()

	if err != nil {
		log.Fatalf("failed to search for unseen messages: %v", err)
	}
	log.Printf("Found %v unseen messages", e.AllSeqNums())

	if len(e.AllSeqNums()) > 0 {

		// fetch all mails
		seqSet := imap.SeqSet{}
		seqSet.AddNum(e.AllSeqNums()...)
		fetchOptions := &imap.FetchOptions{
			Envelope:      true,
			Flags:         true,
			BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
			//Use correct part, you can remove it to find the right part via debugging
			BodySection: []*imap.FetchItemBodySection{{Peek: true, Part: []int{2}}},
		}

		messages, err := c.Fetch(seqSet, fetchOptions).Collect()
		if err != nil {
			log.Fatalf("failed to fetch the last message in INBOX: %v", err)
		}

		if len(messages) > 0 {

			for _, message := range messages {
				logger.Info("Last Message: \n Sender: %s\n Subject: %s\n Date: %s", message.Envelope.From[0].Host, message.Envelope.Subject, message.Envelope.Date.UTC())
				for _, section := range message.BodySection {

					bodyContent := string(section.Bytes)
					logger.Info("Body Content: %s\n", bodyContent)

				}
			}
		}

		// mark fetched mails as seen
		storeFlags := imap.StoreFlags{Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagSeen}, Silent: true}
		if err := c.Store(seqSet, &storeFlags, nil).Close(); err != nil {
			log.Fatalf("failed to mark the last message as seen: %v", err)
		}

	}

	if err := c.Logout().Wait(); err != nil {
		log.Fatalf("failed to logout: %v", err)
	}
}
