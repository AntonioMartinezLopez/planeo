package inbox_test

import (
	"testing"
	"time"

	"planeo/libs/eventschemas"
	domaininbox "planeo/services/core/internal/domain/inbox"
	"planeo/services/core/internal/infra/inbox"

	"github.com/stretchr/testify/assert"
)

func TestMapEmailReceivedEvent(t *testing.T) {
	wire := eventschemas.EmailReceived{
		Subject:        "hi",
		Body:           "body",
		From:           "a@b.com",
		Date:           time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MessageID:      "msg-1",
		OrganizationID: 7,
	}

	got := inbox.MapEmailReceivedEvent(wire)

	want := domaininbox.EmailReceivedEvent{
		Subject:        "hi",
		Body:           "body",
		From:           "a@b.com",
		Date:           time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MessageID:      "msg-1",
		OrganizationID: 7,
	}
	assert.Equal(t, want, got)
}
