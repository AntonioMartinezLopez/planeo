package inbox

import (
	"context"
	"fmt"
	"time"

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

func (s *service) Save(ctx context.Context, topic string, partition int32, offset int64, payload []byte) (bool, error) {
	return s.repo.Save(ctx, topic, partition, offset, payload)
}

func (s *service) FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error) {
	return s.repo.FetchBatch(ctx, topic, instanceID, limit, claimTTL)
}

func (s *service) MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error {
	return s.repo.MarkFailed(ctx, id, procErr, maxAttempts)
}

// ProcessEmailReceived gathers everything the write phase needs (categories,
// LLM extraction, LLM classification) BEFORE opening any transaction — these
// are slow, network-dependent calls, and holding a Postgres row lock and a
// pooled connection across them would be an unnecessary cost. Only the
// domain writes and the inbox row's final status are wrapped together.
func (s *service) ProcessEmailReceived(ctx context.Context, recordID int64, event EmailReceivedEvent, maxAttempts int) error {
	raw := fmt.Sprintf("Subject: %s\nFrom: %s\nDate: %s\nMessage-ID: %s\nBody: %s",
		event.Subject, event.From, event.Date.Format(time.RFC1123), event.MessageID, event.Body)

	categories, err := s.categoryService.GetCategories(ctx, event.OrganizationID)
	if err != nil {
		return s.repo.MarkFailed(ctx, recordID, err, maxAttempts)
	}

	extractedFields, err := s.llmClient.ExtractRequestFields(ctx, raw)
	if err != nil {
		log := logger.FromContext(ctx)
		log.Error().Err(err).Msg("failed to extract fields from request")
		// not fatal - a missing extraction still allows classification and request creation to proceed
	}

	requestData := RequestData{Subject: event.Subject, Text: event.Body}
	categoryId, err := s.llmClient.ClassifyRequest(ctx, requestData, categories)
	if err != nil {
		return s.repo.MarkFailed(ctx, recordID, err, maxAttempts)
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
			Subject:        event.Subject,
			Raw:            raw,
			Text:           event.Body,
			Email:          event.From,
			Name:           extractedFields.Name,
			Address:        extractedFields.Address,
			Telephone:      extractedFields.Phone,
			CategoryId:     categoryIdPtr,
			OrganizationId: event.OrganizationID,
			ReferenceId:    event.MessageID,
		})
		if err != nil {
			return err
		}

		return s.repo.MarkProcessed(txCtx, recordID)
	})
	if txErr != nil {
		return s.repo.MarkFailed(ctx, recordID, txErr, maxAttempts)
	}
	return nil
}
