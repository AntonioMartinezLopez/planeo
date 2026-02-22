package organization

import (
	err "planeo/services/core/pkg/errors"
)

const (
	ErrTypeDomain = "domain"
)

const (
	ErrCodeOrganizationNotFound = iota + 5001000
	ErrCodeInternal
)

var OrganizationNotFoundError = &err.Error{
	Message: "Organization not found",
	Code:    ErrCodeOrganizationNotFound,
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
