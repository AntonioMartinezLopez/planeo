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
	"planeo/services/core/internal/infra/llm"
)

// Repository is implemented once per service (satisfied directly by
// *postgres.Client). topic scopes FetchBatch so that if services/core's
// inbox ever receives a second topic, two consumer adapters can't steal
// each other's rows — mirroring the producer-side outbox.Repository.
type Repository interface {
	FetchBatch(ctx context.Context, topic, instanceID string, limit int, claimTTL time.Duration) ([]libsinbox.Record, error)
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	MarkProcessed(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, procErr error, maxAttempts int) error
}

type EmailReceivedConsumerAdapter struct {
	repo            Repository
	requestService  request.Service
	categoryService category.Service
	topic           string
	instanceID      string
	batchSize       int
	claimTTL        time.Duration
	maxAttempts     int
}

func NewEmailReceivedConsumerAdapter(
	repo Repository,
	requestService request.Service,
	categoryService category.Service,
	topic string,
	instanceID string,
	batchSize int,
	maxAttempts int,
	claimTTL time.Duration,
) *EmailReceivedConsumerAdapter {
	return &EmailReceivedConsumerAdapter{
		repo:            repo,
		requestService:  requestService,
		categoryService: categoryService,
		topic:           topic,
		instanceID:      instanceID,
		batchSize:       batchSize,
		claimTTL:        claimTTL,
		maxAttempts:     maxAttempts,
	}
}

// PollOnce claims a batch of pending inbox rows and processes each in turn.
// One bad record does not stop the rest of the batch.
func (a *EmailReceivedConsumerAdapter) PollOnce(ctx context.Context) error {
	records, err := a.repo.FetchBatch(ctx, a.topic, a.instanceID, a.batchSize, a.claimTTL)
	if err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	for _, rec := range records {
		if err := a.processRecord(ctx, rec); err != nil {
			log.Error().Err(err).Int64("inbox_id", rec.ID).Msg("failed to process inbox record")
		}
	}

	return nil
}

// processRecord gathers everything the write phase needs (categories, LLM
// extraction, LLM classification) BEFORE opening any transaction — these
// are slow, network-dependent calls, and holding a Postgres row lock and a
// pooled connection across them would be an unnecessary cost. Only the
// domain writes and the inbox row's final status are wrapped together.
func (a *EmailReceivedConsumerAdapter) processRecord(ctx context.Context, rec libsinbox.Record) error {
	var payload contracts.EmailCreatedPayload
	if err := json.Unmarshal(rec.Payload, &payload); err != nil {
		return a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
	}

	raw := fmt.Sprintf("Subject: %s\nFrom: %s\nDate: %s\nMessage-ID: %s\nBody: %s",
		payload.Subject, payload.From, payload.Date.Format(time.RFC1123), payload.MessageID, payload.Body)

	categories, err := a.categoryService.GetCategories(ctx, payload.OrganizationId)
	if err != nil {
		return a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
	}

	extractedFields, err := llm.ExtractRequestFields(ctx, raw)
	if err != nil {
		log := logger.FromContext(ctx)
		log.Error().Err(err).Msg("failed to extract fields from request")
		// not fatal - a missing extraction still allows classification and request creation to proceed
	}

	requestData := llm.RequestData{Subject: payload.Subject, Text: payload.Body}
	categoryId, err := llm.ClassifyRequest(ctx, requestData, categories)
	if err != nil {
		return a.repo.MarkFailed(ctx, rec.ID, err, a.maxAttempts)
	}

	// Write-only transaction: CreateRequest/UpdateRequest's writes and
	// MarkProcessed are atomic together. MarkFailed is deliberately NOT
	// called inside this transaction: once any statement in a Postgres
	// transaction errors, the whole transaction is aborted and every
	// subsequent statement on it fails too (SQLSTATE 25P02) — calling
	// MarkFailed(txCtx, ...) after a write error here would itself fail,
	// rolling back and losing the attempts increment entirely.
	txErr := a.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		requestId, err := a.requestService.CreateRequest(txCtx, request.NewRequest{
			Subject:        payload.Subject,
			Raw:            raw,
			Text:           payload.Body,
			Email:          payload.From,
			OrganizationId: payload.OrganizationId,
			ReferenceId:    payload.MessageID,
		})
		if err != nil {
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
		if err := a.requestService.UpdateRequest(txCtx, updatedRequest); err != nil {
			return err
		}

		return a.repo.MarkProcessed(txCtx, rec.ID)
	})
	if txErr != nil {
		return a.repo.MarkFailed(ctx, rec.ID, txErr, a.maxAttempts)
	}
	return nil
}
