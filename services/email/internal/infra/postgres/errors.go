package postgres

import (
	err "planeo/services/email/pkg/errors"
)

const (
	ErrTypeDatabase = "database"
)

const (
	ErrCodeInternal = iota + 6001000
)

func NewDatabaseError(message string, underlyingErr error) *err.Error {
	return &err.Error{
		Message: message,
		Code:    ErrCodeInternal,
		Type:    ErrTypeDatabase,
		Err:     underlyingErr,
	}
}
