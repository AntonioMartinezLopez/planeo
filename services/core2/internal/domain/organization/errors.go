package organization

import (
	"errors"
	err "planeo/services/core2/pkg/errors"
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
	Err:     errors.New("organization not found"),
}

func NewInternalError(msg string, underlyingErr error) *err.Error {
	return &err.Error{
		Message: msg,
		Code:    ErrCodeInternal,
		Type:    ErrTypeDomain,
		Err:     underlyingErr,
	}
}
