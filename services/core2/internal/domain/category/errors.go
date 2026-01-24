package category

import (
	err "planeo/services/core2/pkg/errors"
)

const (
	ErrTypeDomain = "domain"
)

const (
	ErrCodeCategoryNotFound = iota + 5000000
	ErrCodeInternal
)

var CategoryNotFoundError = &err.Error{
	Message: "Category not found",
	Code:    ErrCodeCategoryNotFound,
	Type:    ErrTypeDomain,
}

func NewInternalError(msg string, underlyingErr error) *err.Error {
	return &err.Error{
		Message: msg,
		Code:    ErrCodeInternal,
		Type:    ErrTypeDomain,
		Err:     underlyingErr,
	}
}
