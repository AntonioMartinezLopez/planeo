package errors

type GenericError interface {
	error
	Unwrap() error
}

type Error struct {
	Message string
	Code    int
	Type    string
	Err     error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}
