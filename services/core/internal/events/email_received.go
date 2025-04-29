package events

import (
	"context"
	"errors"
	"fmt"
	"planeo/libs/events"
	"planeo/libs/logger"
	"planeo/services/core/internal/llm"
	"planeo/services/core/internal/resources/request/models"
	"time"
)

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

		requestId, err := e.services.RequestService.CreateRequest(ctx, models.NewRequest{
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

		categories, err := e.services.CategoryService.GetCategories(ctx, payload.OrganizationId)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get categories")
			return err
		}

		// Extract fields from the request
		extractedFields, err := llm.ExtractRequestFields(ctx, raw)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to extract fields from request")
		}

		// Classify the request
		requestData := llm.RequestData{
			Subject: payload.Subject,
			Text:    payload.Body,
		}
		categoryId, err := llm.ClassifyRequest(ctx, requestData, categories)

		if err != nil {
			logger.Error().Err(err).Msg("Failed to classify request")
			return err
		}

		updatedRequest := models.UpdateRequest{
			Id:             requestId,
			Text:           payload.Body,
			Subject:        payload.Subject,
			Email:          payload.From,
			Name:           extractedFields.Name,
			Address:        extractedFields.Address,
			Telephone:      extractedFields.Phone,
			CategoryId:     categoryId,
			OrganizationId: payload.OrganizationId,
		}

		err = e.services.RequestService.UpdateRequest(ctx, updatedRequest)

		if err != nil {
			logger.Error().Err(err).Msg("Failed to update request")
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
