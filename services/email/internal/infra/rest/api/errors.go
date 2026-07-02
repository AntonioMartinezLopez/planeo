package server

import (
	"errors"
	appError "planeo/libs/errors"
	"planeo/services/email/internal/domain/setting"

	"github.com/danielgtaylor/huma/v2"
)

func NewHTTPError(err error) huma.StatusError {
	if errors.Is(err, setting.ErrSettingNotFound) {
		return huma.Error404NotFound("setting not found")
	}

	var e *appError.Error
	if errors.As(err, &e) {
		return huma.NewError(httpStatus(e), e.Message, e.Errors...)
	}

	return huma.Error500InternalServerError(err.Error())
}

func httpStatus(e *appError.Error) int {
	switch e.Type {
	case appError.ValidationError:
		return 400
	case appError.Unauthorized:
		return 401
	case appError.Forbidden:
		return 403
	case appError.EntityNotFound:
		return 404
	default:
		return 500
	}
}
