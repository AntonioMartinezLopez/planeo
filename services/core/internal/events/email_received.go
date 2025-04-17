package events

import (
	"context"
	"errors"
	"fmt"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/core/internal/resources/request/models"
	"time"
)

type NewRequestHandlerInterface interface {
	CreateRequest(ctx context.Context, request models.NewRequest) error
}

func (e *EventService) SubscribeEmailReceived(ctx context.Context) error {
	logger := logger.FromContext(ctx)

	if e.NatsConnector == nil {
		logger.Error().Msg("NATS connector is not initialized")
		return errors.New("NATS connector is not initialized")
	}

	err := e.NatsConnector.SubscribeEmailReceived(ctx, func(payload events.EmailCreatedPayload) error {
		logger.Info().Str("message_id", payload.MessageID).Int("organization_id", payload.OrganizationId).Msg("Received email")

		raw := fmt.Sprintf("Subject: %s\nFrom: %s\nDate: %s\nMessage-ID: %s\nBody: %s",
			payload.Subject, payload.From, payload.Date.Format(time.RFC1123), payload.MessageID, payload.Body)

		err := e.services.RequestService.CreateRequest(ctx, models.NewRequest{
			Subject:        payload.Subject,
			Raw:            raw,
			Text:           payload.Body,
			Email:          payload.From,
			OrganizationId: payload.OrganizationId,
			ReferenceId:    payload.MessageID,
		})

		if err != nil {
			logger.Error().Err(err).Msg("Failed to create request from email")
			return err
		}

		return nil
	})

	if err != nil {
		logger.Error().Err(err).Msg("Failed to subscribe to email.received subject")
		return err
	}

	return nil
}
