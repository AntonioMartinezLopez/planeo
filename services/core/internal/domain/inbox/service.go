package inbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"planeo/libs/events/contracts"
	libsinbox "planeo/libs/inbox"
	"planeo/libs/logger"
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
)

type service struct {
	repo            Repository
	requestService  request.Service
	categoryService category.Service
	llmClient       LLMClient
}

func NewService(
	repo Repository,
	requestService request.Service,
	categoryService category.Service,
	llmClient LLMClient,
) Service {
	return &service{
		repo:            repo,
		requestService:  requestService,
		categoryService: categoryService,
		llmClient:       llmClient,
	}
}

func (s *service) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error) {
	return s.repo.FetchBatch(ctx, topic, instanceID, limit, claimTTL)
}

// ProcessEmailReceived gathers everything the write phase needs (categories,
// LLM extraction, LLM classification) BEFORE opening any transaction — these
// are slow, network-dependent calls, and holding a Postgres row lock and a
// pooled connection across them would be an unnecessary cost. Only the
// domain writes and the inbox row's final status are wrapped together.
func (s *service) ProcessEmailReceived(ctx context.Context, rec libsinbox.Record, maxAttempts int) error {
	var payload contracts.EmailCreatedPayload
	if err := json.Unmarshal(rec.Payload, &payload); err != nil {
		return s.repo.MarkFailed(ctx, rec.ID, err, maxAttempts)
	}

	raw := fmt.Sprintf("Subject: %s\nFrom: %s\nDate: %s\nMessage-ID: %s\nBody: %s",
		payload.Subject, payload.From, payload.Date.Format(time.RFC1123), payload.MessageID, payload.Body)

	categories, err := s.categoryService.GetCategories(ctx, payload.OrganizationId)
	if err != nil {
		return s.repo.MarkFailed(ctx, rec.ID, err, maxAttempts)
	}

	extractedFields, err := s.llmClient.ExtractRequestFields(ctx, raw)
	if err != nil {
		log := logger.FromContext(ctx)
		log.Error().Err(err).Msg("failed to extract fields from request")
		// not fatal - a missing extraction still allows classification and request creation to proceed
	}

	requestData := RequestData{Subject: payload.Subject, Text: payload.Body}
	categoryId, err := s.llmClient.ClassifyRequest(ctx, requestData, categories)
	if err != nil {
		return s.repo.MarkFailed(ctx, rec.ID, err, maxAttempts)
	}

	// Everything the request row needs (raw fields plus the LLM-derived
	// ones) is already known at this point, so a single UpsertRequest
	// replaces what would otherwise be a CreateRequest-then-UpdateRequest
	// two-step — that two-step exists for the REST-facing create path,
	// which doesn't have the LLM-derived fields yet, not for this one.
	var categoryIdPtr *int
	if categoryId != 0 {
		categoryIdPtr = &categoryId
	}

	// Write-only transaction: UpsertRequest's write and MarkProcessed are
	// atomic together. MarkFailed is deliberately NOT called inside this
	// transaction: once any statement in a Postgres transaction errors, the
	// whole transaction is aborted and every subsequent statement on it
	// fails too (SQLSTATE 25P02) — calling MarkFailed(txCtx, ...) after a
	// write error here would itself fail, rolling back and losing the
	// attempts increment entirely.
	txErr := s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		_, err := s.requestService.UpsertRequest(txCtx, request.Request{
			Subject:        payload.Subject,
			Raw:            raw,
			Text:           payload.Body,
			Email:          payload.From,
			Name:           extractedFields.Name,
			Address:        extractedFields.Address,
			Telephone:      extractedFields.Phone,
			CategoryId:     categoryIdPtr,
			OrganizationId: payload.OrganizationId,
			ReferenceId:    payload.MessageID,
		})
		if err != nil {
			return err
		}

		return s.repo.MarkProcessed(txCtx, rec.ID)
	})
	if txErr != nil {
		return s.repo.MarkFailed(ctx, rec.ID, txErr, maxAttempts)
	}
	return nil
}
