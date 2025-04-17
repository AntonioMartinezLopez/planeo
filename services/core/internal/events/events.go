package events

import (
	"context"
	"errors"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/core/internal/resources/request/models"
)

type RequestServiceInterface interface {
	CreateRequest(ctx context.Context, request models.NewRequest) error
}

type Services struct {
	RequestService RequestServiceInterface
}

type EventService struct {
	NatsConnector *events.NatsConnector
	services      Services
}

func NewEventService(natsConnector *events.NatsConnector) *EventService {
	return &EventService{
		NatsConnector: natsConnector,
	}
}

func (e *EventService) InitializeEvents(ctx context.Context, services Services) error {
	logger := logger.FromContext(ctx)
	e.services = services

	if e.NatsConnector == nil {
		logger.Error().Msg("NATS connector is not initialized")
		return errors.New("NATS connector is not initialized")
	}

	err := e.SubscribeEmailReceived(ctx)

	if err != nil {
		logger.Error().Err(err).Msg("Failed to subscribe to email.received subject")
		return err
	}

	return nil
}
