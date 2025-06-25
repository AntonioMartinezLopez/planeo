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

type EventServiceInterface interface {
	SubscribeEmailReceived(ctx context.Context, callback func(payload events.EmailCreatedPayload) error) error
	IsConnected() bool
}

type Services struct {
	RequestService  RequestServiceInterface
	CategoryService CategoryServiceInterface
}

func InitializeEvents(ctx context.Context, eventService EventServiceInterface, services Services) error {
	logger := logger.FromContext(ctx)

	if !eventService.IsConnected() {
		logger.Error().Msg("events are not initialized")
		return errors.New("events are not initialized")
	}

	err := eventService.SubscribeEmailReceived(ctx, CreateEmailReceivedCallback(ctx, services))

	if err != nil {
		logger.Error().Err(err).Msg("Failed to subscribe to email.received subject")
		return err
	}

	return nil
}
