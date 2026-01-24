package request

import (
	"errors"
	err "planeo/services/core2/pkg/errors"
)

const (
	ErrTypeDomain = "domain"
)

const (
	ErrCodeRequestNotFound = iota + 5001000
	ErrCodeInternal
)

var RequestNotFoundError = &err.Error{
	Message: "Request not found",
	Code:    ErrCodeRequestNotFound,
	Type:    ErrTypeDomain,
	Err:     errors.New("request not found"),
}

func NewInternalError(msg string, underlyingErr error) *err.Error {
	return &err.Error{
		Message: msg,
		Code:    ErrCodeInternal,
		Type:    ErrTypeDomain,
		Err:     underlyingErr,
	}
}
