package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// EmailCheckJobPayload represents a scheduled email check task
type EmailCheckJobPayload struct {
	SettingID      int    `json:"settingId"`
	OrganizationID int    `json:"organizationId"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	ScheduledAt    time.Time `json:"scheduledAt"`
}

var emailCheckJobSubject = "jobs.email.check"
var emailCheckJobConsumer = "email-check-workers"

// PublishEmailCheckJob publishes a job to check an email account
func (nc *EventService) PublishEmailCheckJob(ctx context.Context, payload EmailCheckJobPayload) error {
	payload.ScheduledAt = time.Now()
	
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return nc.Publish(ctx, emailCheckJobSubject, data)
}

// SubscribeEmailCheckJob subscribes to email check jobs with work queue semantics
// Multiple workers can subscribe - each job goes to exactly one worker
func (nc *EventService) SubscribeEmailCheckJob(ctx context.Context, handler func(EmailCheckJobPayload) error) error {
	return nc.Subscribe(ctx, emailCheckJobConsumer, emailCheckJobSubject, func(msg jetstream.Msg) {
		var payload EmailCheckJobPayload
		if err := json.Unmarshal(msg.Data(), &payload); err != nil {
			// Invalid payload - ack to remove from queue
			_ = msg.Ack()
			return
		}

		err := handler(payload)
		if err != nil {
			// Retry with exponential backoff
			_ = msg.NakWithDelay(1 * time.Minute)
			return
		}

		// Success - remove from queue
		_ = msg.Ack()
	})
}
