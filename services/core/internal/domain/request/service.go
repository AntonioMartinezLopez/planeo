package request

import "context"

type service struct {
	requestRepository RequestRepository
}

func NewService(requestRepository RequestRepository) Service {
	return &service{
		requestRepository: requestRepository,
	}
}

func (s *service) GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool, selectedCategories []int) ([]Request, error) {
	return s.requestRepository.GetRequests(ctx, organizationId, cursor, limit, getClosed, selectedCategories)
}

func (s *service) GetRequest(ctx context.Context, organizationId int, requestId int) (Request, error) {
	return s.requestRepository.GetRequest(ctx, organizationId, requestId)
}

func (s *service) CreateRequest(ctx context.Context, request NewRequest) (int, error) {
	return s.requestRepository.CreateRequest(ctx, request)
}

func (s *service) UpdateRequest(ctx context.Context, request UpdateRequest) error {
	return s.requestRepository.UpdateRequest(ctx, request)
}

func (s *service) DeleteRequest(ctx context.Context, organizationId int, requestId int) error {
	return s.requestRepository.DeleteRequest(ctx, organizationId, requestId)
}
