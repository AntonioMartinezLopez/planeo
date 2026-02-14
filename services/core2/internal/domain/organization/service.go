package organization

import "context"

type service struct {
	organizationRepository OrganizationRepository
}

func NewService(organizationRepository OrganizationRepository) Service {
	return &service{
		organizationRepository: organizationRepository,
	}
}

func (s *service) GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error) {
	return s.organizationRepository.GetOrganizationsByUserSub(ctx, userSub)
}

func (s *service) GetOrganizationById(ctx context.Context, organizationId int) (Organization, error) {
	return s.organizationRepository.GetOrganizationById(ctx, organizationId)
}
