package events

import (
	"context"
	"encoding/json"
	"time"
)

type EmailCreatedPayload struct {
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	From           string    `json:"from"`
	Date           time.Time `json:"date"`
	MessageID      string    `json:"messageId"`
	OrganizationId int       `json:"organizationId"`
}

var topic = "email-received"
var subscriptionName = "email-receiver"

func (es *EventService) PublishEmailReceived(ctx context.Context, payload EmailCreatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return es.Publish(ctx, topic, data)
}

func (es *EventService) SubscribeEmailReceived(ctx context.Context, handler func(EmailCreatedPayload) error) error {
	return es.Subscribe(ctx, subscriptionName, topic, func(data []byte) error {
		var payload EmailCreatedPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			return err
		}

		return handler(payload)
	})
}
