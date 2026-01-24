package user

import (
	err "planeo/services/core2/pkg/errors"
)

const (
	ErrTypeDomain = "domain"
)

const (
	ErrCodeUserNotFound = iota + 5003000
	ErrCodeInternal
)

var UserNotFoundError = &err.Error{
	Message: "User not found",
	Code:    ErrCodeUserNotFound,
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
