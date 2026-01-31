package request

import (
	"context"
)

type RequestRepository interface {
	CreateRequest(ctx context.Context, request NewRequest) (int, error)
	GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool, selectedCategories []int) ([]Request, error)
	GetRequest(ctx context.Context, organizationId int, requestId int) (Request, error)
	UpdateRequest(ctx context.Context, request UpdateRequest) error
	DeleteRequest(ctx context.Context, organizationId int, requestId int) error
}
