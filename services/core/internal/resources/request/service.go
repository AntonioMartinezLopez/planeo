package request

import (
	"context"
	"planeo/services/core/internal/resources/request/models"
)

type RequestRepositoryInterface interface {
	CreateRequest(ctx context.Context, request models.NewRequest) error
	GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool) ([]models.Request, error)
	UpdateRequest(ctx context.Context, request models.UpdateRequest) error
	DeleteRequest(ctx context.Context, organizationId int, requestId int) error
}

type RequestService struct {
	requestRepository RequestRepositoryInterface
}

func NewRequestService(requestRepository RequestRepositoryInterface) *RequestService {
	return &RequestService{
		requestRepository: requestRepository,
	}
}

func (s *RequestService) GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool) ([]models.Request, error) {
	return s.requestRepository.GetRequests(ctx, organizationId, cursor, limit, getClosed)
}

func (s *RequestService) CreateRequest(ctx context.Context, request models.NewRequest) error {
	return s.requestRepository.CreateRequest(ctx, request)
}

func (s *RequestService) UpdateRequest(ctx context.Context, request models.UpdateRequest) error {
	return s.requestRepository.UpdateRequest(ctx, request)
}

func (s *RequestService) DeleteRequest(ctx context.Context, organizationId int, requestId int) error {
	return s.requestRepository.DeleteRequest(ctx, organizationId, requestId)
}
