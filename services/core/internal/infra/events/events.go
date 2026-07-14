package core_events

import (
	"planeo/services/core/internal/domain/category"
	"planeo/services/core/internal/domain/request"
)

type Services struct {
	RequestService  request.Service
	CategoryService category.Service
}
