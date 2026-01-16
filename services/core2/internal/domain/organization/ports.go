package organization

import (
	"context"
)

type OrganizationRepositoryInterface interface {
	GetOrganizationsByUserSub(ctx context.Context, userSub string) ([]Organization, error)
}
