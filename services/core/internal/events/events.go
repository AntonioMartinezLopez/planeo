package events

import (
	"context"
	"errors"
	"planeo/libs/events"
	"planeo/libs/logger"
	categories "planeo/services/core/internal/resources/category/models"
	requests "planeo/services/core/internal/resources/request/models"
)

type RequestServiceInterface interface {
	GetRequest(ctx context.Context, organizationId int, requestId int) (requests.Request, error)
	CreateRequest(ctx context.Context, request requests.NewRequest) (int, error)
	UpdateRequest(ctx context.Context, request requests.UpdateRequest) error
}

type CategoryServiceInterface interface {
	GetCategories(ctx context.Context, organizationId int) ([]categories.Category, error)
}

type Services struct {
	RequestService  RequestServiceInterface
	CategoryService CategoryServiceInterface
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
