package errdefs

import (
	"errors"
	"fmt"
)

// ErrUnsupported is an error interface which denotes whether the operation failed due
// to the operation being unsupported. It will not succeed on subsequent retries.
type ErrUnsupported interface {
	Unsupported() bool
	error
}

type unsupportedError struct {
	error
}

func (e *unsupportedError) Unsupported() bool {
	return true
}

func (e *unsupportedError) Cause() error {
	return e.error
}

// AsUnsupported wraps the passed in error to make it of type ErrUnsupported
//
// Callers should make sure the passed in error has exactly the error message
// it wants as this function does not decorate the message.
func AsUnsupported(err error) error {
	if err == nil {
		return nil
	}
	return &unsupportedError{err}
}

// Unsupported makes an ErrUnsupported from the provided error message
func Unsupported(msg string) error {
	return &unsupportedError{errors.New(msg)}
}

// Unsupportedf makes an ErrUnsupported from the provided error format and args
func Unsupportedf(format string, args ...interface{}) error {
	return &unsupportedError{fmt.Errorf(format, args...)}
}

// IsUnsupported determines if the passed in error is of type ErrUnsupported
//
// This will traverse the causal chain (`Cause() error`), until it finds an error
// which implements the `NotFound` interface.
func IsUnsupported(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(ErrUnsupported); ok {
		return e.Unsupported()
	}

	if e, ok := err.(causal); ok {
		return IsUnsupported(e.Cause())
	}

	return false
}
