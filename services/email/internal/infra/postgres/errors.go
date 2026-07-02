package postgres

import appError "planeo/libs/errors"

func NewDatabaseError(message string, underlyingErr error) *appError.Error {
	return appError.New(appError.InternalError, message, underlyingErr)
}
