package errdefs

import (
	"errors"
	"fmt"
)

var (
	// ErrForbidden is returned when the user is not authorized to perform the operation.
	ErrForbidden = errors.New("forbidden")
	// ErrUnauthorized is returned when the user is not authenticated.
	ErrUnauthorized = errors.New("unauthorized")
)

// Unauthorized wraps ErrUnauthorized with a message.
func Unauthorized(msg string) error {
	return fmt.Errorf("%w: %s", ErrUnauthorized, msg)
}

// Forbidden wraps ErrForbidden with a message.
func Forbidden(msg string) error {
	return fmt.Errorf("%w: %s", ErrForbidden, msg)
}

// IsForbidden returns true if the error has ErrForbidden in the error chain.
func IsForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// IsUnauthorized returns true if the error has ErrUnauthorized in the error chain.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}
