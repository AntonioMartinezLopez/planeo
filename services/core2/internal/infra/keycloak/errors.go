package keycloak

import (
	err "planeo/services/core2/pkg/errors"
)

const (
	ErrTypeKeycloak = "keycloak"
)

const (
	ErrCodeKeycloak = iota + 5005000
)

func NewKeycloakError(msg string, underlyingErr error) *err.Error {
	return &err.Error{
		Message: msg,
		Code:    ErrCodeKeycloak,
		Type:    ErrTypeKeycloak,
		Err:     underlyingErr,
	}
}
