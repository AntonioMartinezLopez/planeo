package organization

import (
	"context"
	"planeo/services/core/internal/resources/organization/models"
)

type OrganizationRepositoryInterface interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]models.Organization, error)
}

type OrganizationService struct {
	organizationRepository OrganizationRepositoryInterface
}

func NewOrganizationService(organizationRepository OrganizationRepositoryInterface) *OrganizationService {
	return &OrganizationService{
		organizationRepository: organizationRepository,
	}
}

func (s *OrganizationService) GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]models.Organization, error) {
	return s.organizationRepository.GetOrganizationsByUserSub(ctx, userSub)
}
