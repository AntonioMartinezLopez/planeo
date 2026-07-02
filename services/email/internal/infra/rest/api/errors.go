package server

import (
	"errors"
	emailInfra "planeo/services/email/internal/infra/email"
	"planeo/services/email/internal/domain/setting"
	"planeo/services/email/internal/infra/postgres"
	err "planeo/services/email/pkg/errors"

	"github.com/danielgtaylor/huma/v2"
)

func NewHTTPError(unknownErr error) huma.StatusError {
	var appError *err.Error
	if errors.As(unknownErr, &appError) {
		switch appError.Type {
		case setting.ErrTypeDomain:
			switch appError.Code {
			case setting.ErrCodeSettingNotFound:
				return huma.Error404NotFound(appError.Message, appError.Unwrap())
			case setting.ErrCodeInternal:
				return huma.Error500InternalServerError(appError.Message, appError.Unwrap())
			default:
				return huma.Error500InternalServerError(appError.Message, appError.Unwrap())
			}
		case postgres.ErrTypeDatabase:
			return huma.Error500InternalServerError(appError.Message, appError.Unwrap())
		case emailInfra.ErrTypeIMAP:
			return huma.Error400BadRequest(appError.Message, appError.Unwrap())
		default:
			return huma.Error500InternalServerError(appError.Message, appError.Unwrap())
		}
	}
	return huma.Error500InternalServerError(unknownErr.Error())
}
