package organization

import (
	"context"
)

type OrganizationRepository interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error)
}

type Service interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error)
}
