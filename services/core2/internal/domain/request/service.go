package request

import "context"

type Service struct {
	requestRepository RequestRepositoryInterface
}

func NewService(requestRepository RequestRepositoryInterface) *Service {
	return &Service{
		requestRepository: requestRepository,
	}
}

func (s *Service) GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool, selectedCategories []int) ([]Request, error) {
	return s.requestRepository.GetRequests(ctx, organizationId, cursor, limit, getClosed, selectedCategories)
}

func (s *Service) GetRequest(ctx context.Context, organizationId int, requestId int) (Request, error) {
	return s.requestRepository.GetRequest(ctx, organizationId, requestId)
}

func (s *Service) CreateRequest(ctx context.Context, request NewRequest) (int, error) {
	return s.requestRepository.CreateRequest(ctx, request)
}

func (s *Service) UpdateRequest(ctx context.Context, request UpdateRequest) error {
	return s.requestRepository.UpdateRequest(ctx, request)
}

func (s *Service) DeleteRequest(ctx context.Context, organizationId int, requestId int) error {
	return s.requestRepository.DeleteRequest(ctx, organizationId, requestId)
}
