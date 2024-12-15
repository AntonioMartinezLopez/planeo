package err

type ErrorType string

const (
	ValidationError ErrorType = "VALIDATION_ERROR"
	Unauthorized    ErrorType = "UNAUTHORIZED"
	Forbidden       ErrorType = "FORBIDDEN"
	EntityNotFound  ErrorType = "ENTITY_NOT_FOUND"
	InternalError   ErrorType = "INTERNAL_ERROR"
)

type Error struct {
	Type    ErrorType
	Message string
	Errors  []error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) ErrorDetail() ErrorType {
	return e.Type
}

func New(errorType ErrorType, message string, errs ...error) *Error {
	return &Error{
		Type:    errorType,
		Message: message,
		Errors:  errs,
	}
}
