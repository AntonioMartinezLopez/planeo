package organization

import "context"

type Service struct {
	organizationRepository OrganizationRepository
}

func NewService(organizationRepository OrganizationRepository) *Service {
	return &Service{
		organizationRepository: organizationRepository,
	}
}

func (s *Service) GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error) {
	return s.organizationRepository.GetOrganizationsByUserSub(ctx, userSub)
}
