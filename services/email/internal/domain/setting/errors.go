package setting

import (
	err "planeo/services/email/pkg/errors"
)

const (
	ErrTypeDomain = "domain"
)

const (
	ErrCodeSettingNotFound = iota + 6000000
	ErrCodeInternal
)

var SettingNotFoundError = &err.Error{
	Message: "Setting not found",
	Code:    ErrCodeSettingNotFound,
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
