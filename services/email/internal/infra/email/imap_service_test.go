package email

import (
	"context"
	"strings"
	"testing"

	"github.com/emersion/go-message/mail"
	"github.com/stretchr/testify/assert"
)

func TestExtractHeaderFields(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	s := &IMAPService{}

	t.Run("returns the sender address when a From header is present", func(t *testing.T) {
		raw := "From: Alice <alice@example.com>\r\n" +
			"Subject: Hello\r\n" +
			"Date: Mon, 1 Jan 2024 12:00:00 +0000\r\n" +
			"Message-Id: <test-with-from@example.com>\r\n" +
			"\r\n" +
			"Body text.\r\n"

		mr, err := mail.CreateReader(strings.NewReader(raw))
		assert.Nil(t, err)

		email, err := s.extractHeaderFields(context.Background(), mr.Header, Email{})
		assert.Nil(t, err)
		assert.Equal(t, "alice@example.com", email.From)
	})

	t.Run("does not panic and leaves From empty when the From header is missing", func(t *testing.T) {
		raw := "Subject: Hello\r\n" +
			"Date: Mon, 1 Jan 2024 12:00:00 +0000\r\n" +
			"Message-Id: <test-without-from@example.com>\r\n" +
			"\r\n" +
			"Body text.\r\n"

		mr, err := mail.CreateReader(strings.NewReader(raw))
		assert.Nil(t, err)

		email, err := s.extractHeaderFields(context.Background(), mr.Header, Email{})
		assert.Nil(t, err)
		assert.Equal(t, "", email.From)
	})
}
