package category

import (
	"errors"
	err "planeo/services/core2/pkg/errors"
)

const (
	ErrTypeDomain   = "domain"
	ErrTypeDatabase = "database"
)

const (
	ErrCodeCategoryNotFound = iota + 5001000
	ErrCodeInternal
)

var CategoryNotFoundError = err.Error{
	Message: "Category not found",
	Code:    ErrCodeCategoryNotFound,
	Type:    ErrTypeDomain,
	Err:     errors.New("category not found"),
}

func NewInternalError(msg string, underlyingErr error) *err.Error {
	return &err.Error{
		Message: msg,
		Code:    ErrCodeInternal,
		Type:    ErrTypeDomain,
		Err:     underlyingErr,
	}
}
