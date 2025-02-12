package request

import (
	"context"
	"planeo/api/internal/resources/request/models"
)

type RequestRepositoryInterface interface {
	GetRequests(ctx context.Context, organizationId string, cursor int, limit int, getClosed bool) ([]models.Request, error)
}

type RequestService struct {
	requestRepository RequestRepositoryInterface
}

func NewRequestService(requestRepository RequestRepositoryInterface) *RequestService {
	return &RequestService{
		requestRepository: requestRepository,
	}
}

func (s *RequestService) GetRequests(ctx context.Context, organizationId string, cursor int, limit int, getClosed bool) ([]models.Request, error) {
	return s.requestRepository.GetRequests(ctx, organizationId, cursor, limit, getClosed)
}

func (s *RequestService) CreateRequest() string {
	return "CreateRequest endpoint"
}

func (s *RequestService) UpdateRequest(id string) string {
	return "UpdateRequest endpoint"
}

func (s *RequestService) DeleteRequest(id string) string {
	return "DeleteRequest endpoint"
}
