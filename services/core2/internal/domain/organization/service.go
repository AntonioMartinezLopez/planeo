package organization

import "context"

type Service struct {
	organizationRepository OrganizationRepositoryInterface
}

func NewService(organizationRepository OrganizationRepositoryInterface) *Service {
	return &Service{
		organizationRepository: organizationRepository,
	}
}

func (s *Service) GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error) {
	return s.organizationRepository.GetOrganizationsByUserSub(ctx, userSub)
}
