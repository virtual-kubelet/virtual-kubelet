package providers

import (
	"time"
)

// RetryableError is used to indicate that the operation which caused the error is retryable
type RetryableError interface {
	error
	RetryAfter() time.Duration
}

// Retryable is the struct for a retryable error
type Retryable struct {
	cause      error
	retryAfter time.Duration
}

// Error implements the Error interface
func (e *Retryable) Error() string {
	return e.cause.Error()
}

// Cause implements the cause interface
func (e *Retryable) Cause() error {
	return e.cause
}

// RetryAfter implements the RetryableError interface
func (e *Retryable) RetryAfter() time.Duration {
	return e.retryAfter
}

// NewRetryableError creates a retryable error
func NewRetryableError(err error, retryAfter time.Duration) error {
	return &Retryable{cause: err, retryAfter: retryAfter}
}

// IsRetryable checks whether the error is retryable
func IsRetryable(err error) (time.Duration, bool) {
	e, ok := err.(RetryableError)
	if !ok {
		return time.Duration(0), false
	}

	return e.RetryAfter(), true
}