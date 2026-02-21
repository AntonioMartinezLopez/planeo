package core_events

import (
	"context"
	"errors"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
)

type Services struct {
	RequestService  request.Service
	CategoryService category.Service
}

func InitializeEvents(ctx context.Context, messengerUrl string, services Services) error {
	eventService, err := events.NewEventService(messengerUrl)
	if err != nil {
		return err
	}

	logger := logger.FromContext(ctx)

	if !eventService.IsConnected() {
		logger.Error().Msg("events are not initialized")
		return errors.New("events are not initialized")
	}

	err = eventService.SubscribeEmailReceived(ctx, CreateEmailReceivedCallback(ctx, services))

	if err != nil {
		logger.Error().Err(err).Msg("Failed to subscribe to email.received subject")
		return err
	}

	return nil
}
