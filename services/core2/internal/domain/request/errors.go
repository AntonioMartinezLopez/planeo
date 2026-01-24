package request

import (
	err "planeo/services/core2/pkg/errors"
)

const (
	ErrTypeDomain = "domain"
)

const (
	ErrCodeRequestNotFound = iota + 5002000
	ErrCodeInternal
)

var RequestNotFoundError = &err.Error{
	Message: "Request not found",
	Code:    ErrCodeRequestNotFound,
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
