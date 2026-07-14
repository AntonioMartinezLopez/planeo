package core_events

import (
	"context"
	"encoding/json"
	"fmt"
	"planeo/libs/events/contracts"
	"planeo/libs/inbox"
	"planeo/libs/logger"
	"time"

	"planeo/services/core/internal/domain/request"
	"planeo/services/core/internal/infra/llm"
)

//nolint:funlen
func CreateInboxHandler(services Services) inbox.Handler {
	return func(ctx context.Context, record inbox.Record) error {
		log := logger.FromContext(ctx)

		var payload contracts.EmailCreatedPayload
		if err := json.Unmarshal(record.Payload, &payload); err != nil {
			log.Error().Err(err).Int64("inbox_id", record.ID).Msg("Failed to unmarshal inbox payload")
			return err
		}

		log.Info().Str("message_id", payload.MessageID).Int("organization_id", payload.OrganizationId).Msg("Received email")

		raw := fmt.Sprintf("Subject: %s\nFrom: %s\nDate: %s\nMessage-ID: %s\nBody: %s",
			payload.Subject, payload.From, payload.Date.Format(time.RFC1123), payload.MessageID, payload.Body)

		requestId, err := services.RequestService.CreateRequest(ctx, request.NewRequest{
			Subject:        payload.Subject,
			Raw:            raw,
			Text:           payload.Body,
			Email:          payload.From,
			OrganizationId: payload.OrganizationId,
			ReferenceId:    payload.MessageID,
		})

		if err != nil {
			log.Error().Err(err).Msg("Failed to create request from email")
			return err
		}

		categories, err := services.CategoryService.GetCategories(ctx, payload.OrganizationId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get categories")
			return err
		}

		// Extract fields from the request
		extractedFields, err := llm.ExtractRequestFields(ctx, raw)
		if err != nil {
			log.Error().Err(err).Msg("Failed to extract fields from request")
		}

		// Classify the request
		requestData := llm.RequestData{
			Subject: payload.Subject,
			Text:    payload.Body,
		}
		categoryId, err := llm.ClassifyRequest(ctx, requestData, categories)

		if err != nil {
			log.Error().Err(err).Msg("Failed to classify request")
			return err
		}

		updatedRequest := request.UpdateRequest{
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

		err = services.RequestService.UpdateRequest(ctx, updatedRequest)

		if err != nil {
			log.Error().Err(err).Msg("Failed to update request")
			return err
		}

		return nil
	}
}
