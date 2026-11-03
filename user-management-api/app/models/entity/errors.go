package entity

import "errors"

// Business errors. Handlers translate these into response codes and HTTP
// or gRPC statuses; the core never knows about transports.
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidName        = errors.New("name must be 1-100 characters")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrInvalidPassword    = errors.New("password must be at least 8 characters")

	// ErrInvalidCredentials is deliberately vague: it never reveals whether
	// the email exists or the password was wrong.
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// ValidationError reports a missing or malformed request field, e.g.
// "email is required".
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// NewValidationError returns a *ValidationError with the given message.
func NewValidationError(msg string) error { return &ValidationError{Message: msg} }
