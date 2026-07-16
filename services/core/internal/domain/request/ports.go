package request

import (
	"context"
)

type RequestRepository interface {
	CreateRequest(ctx context.Context, request NewRequest) (int, error)
	// UpsertRequest writes every column of req in a single statement, keyed
	// on (organization_id, reference_id). Callers that already have the full
	// Request in hand (e.g. inbox processing, which gathers LLM-derived
	// fields before writing) should use this instead of CreateRequest
	// followed by UpdateRequest.
	UpsertRequest(ctx context.Context, req Request) (int, error)
	GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool, selectedCategories []int) ([]Request, error)
	GetRequest(ctx context.Context, organizationId int, requestId int) (Request, error)
	UpdateRequest(ctx context.Context, request UpdateRequest) error
	DeleteRequest(ctx context.Context, organizationId int, requestId int) error
}

type Service interface {
	CreateRequest(ctx context.Context, request NewRequest) (int, error)
	UpsertRequest(ctx context.Context, req Request) (int, error)
	GetRequests(ctx context.Context, organizationId int, cursor int, limit int, getClosed bool, selectedCategories []int) ([]Request, error)
	GetRequest(ctx context.Context, organizationId int, requestId int) (Request, error)
	UpdateRequest(ctx context.Context, request UpdateRequest) error
	DeleteRequest(ctx context.Context, organizationId int, requestId int) error
}
