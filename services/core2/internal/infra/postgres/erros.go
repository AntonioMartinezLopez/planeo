package postgres

import (
	err "planeo/services/core2/pkg/errors"
)

const (
	ErrTypeDatabase = "database"
)

const (
	ErrCodeCategoryNotFound = iota + 5001000
	ErrCodeInternal
)

func NewDatabaseError(message string, underlyingErr error) *err.Error {
	return &err.Error{
		Message: message,
		Code:    ErrCodeInternal,
		Type:    ErrTypeDatabase,
		Err:     underlyingErr,
	}
}
