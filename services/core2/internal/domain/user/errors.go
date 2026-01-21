package user

type Error struct {
	Message string
	Code    int
	Type    string
}

func (e *Error) Error() string {
	return e.Message
}

const (
	ErrTypeIAM      = "iam"
	ErrTypeDomain   = "domain"
	ErrTypeDatabase = "database"
)

const (
	ErrCodeUserNotFound = iota + 5001000
	ErrCodeInternal
	ErrCodeDatabaseInternal
)

var (
	UserNotFoundErr = &Error{
		Message: "User not found",
		Code:    ErrCodeUserNotFound,
		Type:    ErrTypeDomain,
	}
	ErrDatabaseInternal = &Error{
		Message: "internal database error",
		Code:    ErrCodeDatabaseInternal,
		Type:    ErrTypeDatabase,
	}
	ErrIamProviderInternal = &Error{
		Message: "internal IAM provider error",
		Code:    ErrCodeInternal,
		Type:    ErrTypeIAM,
	}
)
