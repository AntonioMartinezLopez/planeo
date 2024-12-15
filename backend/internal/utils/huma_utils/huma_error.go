package huma_utils

import (
	"errors"
	appError "planeo/api/internal/errors"

	"github.com/danielgtaylor/huma/v2"
)

func GetStatus(e *appError.Error) int {
	switch e.Type {
	case appError.ValidationError:
		return 400
	case appError.Unauthorized:
		return 401
	case appError.Forbidden:
		return 403
	case appError.EntityNotFound:
		return 404
	case appError.InternalError:
		return 500
	default:
		return 500
	}
}

func NewHumaError(unknown_err error) huma.StatusError {

	var appError *appError.Error
	if errors.As(unknown_err, &appError) {
		return huma.NewError(GetStatus(appError), appError.Message, appError.Errors...)
	}

	return huma.Error500InternalServerError(appError.Error())

}
