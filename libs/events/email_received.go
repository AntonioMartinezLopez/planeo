package events

import (
	"context"
	"encoding/json"
	"planeo/libs/events/contracts"
)

var subscriptionName = "email-receiver"

func (es *EventService) PublishEmailReceived(ctx context.Context, payload contracts.EmailCreatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return es.Publish(ctx, contracts.EmailReceivedTopic, data)
}

func (es *EventService) SubscribeEmailReceived(ctx context.Context, handler func(contracts.EmailCreatedPayload) error) error {
	return es.Subscribe(ctx, subscriptionName, contracts.EmailReceivedTopic, func(data []byte) error {
		var payload contracts.EmailCreatedPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			return err
		}

		return handler(payload)
	})
}
