package email

import (
	err "planeo/services/email/pkg/errors"
)

const (
	ErrTypeIMAP = "imap"
)

const (
	ErrCodeInvalidPort = iota + 6002000
)

func NewIMAPError(message string, underlyingErr error) *err.Error {
	return &err.Error{
		Message: message,
		Code:    ErrCodeInvalidPort,
		Type:    ErrTypeIMAP,
		Err:     underlyingErr,
	}
}
