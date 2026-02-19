package server

import (
	"errors"
	"planeo/services/core2/internal/domain/category"
	"planeo/services/core2/internal/domain/organization"
	"planeo/services/core2/internal/domain/request"
	"planeo/services/core2/internal/domain/user"
	"planeo/services/core2/internal/infra/keycloak"
	err "planeo/services/core2/pkg/errors"

	"github.com/danielgtaylor/huma/v2"
)

func NewHTTPError(unknownErr error) huma.StatusError {
	var appError *err.Error
	if errors.As(unknownErr, &appError) {
		switch appError.Type {
		case category.ErrTypeDomain:
			switch appError.Code {
			case
				category.ErrCodeCategoryNotFound,
				organization.ErrCodeOrganizationNotFound,
				user.ErrCodeUserNotFound,
				request.ErrCodeRequestNotFound:
				return huma.Error404NotFound(appError.Message, appError.Unwrap())
			case
				category.ErrCodeInternal,
				organization.ErrCodeInternal,
				user.ErrCodeInternal,
				request.ErrCodeInternal:
				return huma.Error500InternalServerError(appError.Message, appError.Unwrap())
			default:
				return huma.Error500InternalServerError(appError.Message, appError.Unwrap())
			}
		case keycloak.ErrTypeKeycloak:
			switch appError.Code {
			case keycloak.ErrCodeKeycloakUserNotFound:
				return huma.Error404NotFound(appError.Message, appError.Unwrap())
			case keycloak.ErrCodeKeycloak:
				return huma.Error500InternalServerError(appError.Message, appError.Unwrap())
			default:
				return huma.Error500InternalServerError(appError.Message, appError.Unwrap())
			}
		default:
			return huma.Error500InternalServerError(appError.Message, appError.Unwrap())
		}
	}

	return huma.Error500InternalServerError(unknownErr.Error())
}
