package organization

import (
	"context"
)

type OrganizationRepository interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error)
	GetOrganizationById(ctx context.Context, id int) (Organization, error)
	GetOrganizationByIAMId(ctx context.Context, iamOrganizationId string) (Organization, error)
}

type Service interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error)
	GetOrganizationById(ctx context.Context, organizationId int) (Organization, error)
	GetOrganizationByIAMId(ctx context.Context, iamOrganizationId string) (Organization, error)
}
