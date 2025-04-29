package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type EmailCreatedPayload struct {
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	From           string    `json:"from"`
	Date           time.Time `json:"date"`
	MessageID      string    `json:"messageId"`
	OrganizationId int       `json:"organizationId"`
}

var subject = "events.email.received"
var subscriptionName = "email-receiver"

func (nc *NatsConnector) PublishEmailReceived(ctx context.Context, payload EmailCreatedPayload) error {
	json, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	err = nc.Publish(ctx, subject, json)

	if err != nil {
		return err
	}

	return nil
}

func (nc *NatsConnector) SubscribeEmailReceived(ctx context.Context, handler func(EmailCreatedPayload) error) error {

	err := nc.Subscribe(ctx, subscriptionName, subject, func(msg jetstream.Msg) {
		var payload EmailCreatedPayload
		if err := json.Unmarshal(msg.Data(), &payload); err != nil {
			return
		}
		defer msg.Ack()

		err := handler(payload)
		if err != nil {
			return
		}
	})

	if err != nil {
		return err
	}

	return nil
}
