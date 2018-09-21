// Package ocstatus provides error status conversions to opencencus status trace.StatusCode
package ocstatus

import (
	"github.com/cpuguy83/strongerrors"
	"go.opencensus.io/trace"
)

// FromError makes an opencencus trace.Status from the passed in error.
func FromError(err error) trace.Status {
	if err == nil {
		return trace.Status{Code: trace.StatusCodeOK}
	}

	switch {
	case strongerrors.IsNotFound(err):
		return status(trace.StatusCodeNotFound, err)
	case strongerrors.IsConflict(err), strongerrors.IsNotModified(err):
		return status(trace.StatusCodeFailedPrecondition, err)
	case strongerrors.IsInvalidArgument(err):
		return status(trace.StatusCodeInvalidArgument, err)
	case strongerrors.IsAlreadyExists(err):
		return status(trace.StatusCodeAlreadyExists, err)
	case strongerrors.IsCancelled(err):
		return status(trace.StatusCodeCancelled, err)
	case strongerrors.IsDeadline(err):
		return status(trace.StatusCodeDeadlineExceeded, err)
	case strongerrors.IsUnauthorized(err):
		return status(trace.StatusCodePermissionDenied, err)
	case strongerrors.IsUnauthenticated(err):
		return status(trace.StatusCodeUnauthenticated, err)
	case strongerrors.IsForbidden(err), strongerrors.IsNotImplemented(err):
		return status(trace.StatusCodeUnimplemented, err)
	case strongerrors.IsExhausted(err):
		return status(trace.StatusCodeResourceExhausted, err)
	case strongerrors.IsDataLoss(err):
		return status(trace.StatusCodeDataLoss, err)
	case strongerrors.IsSystem(err):
		return status(trace.StatusCodeInternal, err)
	case strongerrors.IsUnavailable(err):
		return status(trace.StatusCodeUnavailable, err)
	default:
		return status(trace.StatusCodeUnknown, err)
	}
}

func status(code int32, err error) trace.Status {
	return trace.Status{Code: code, Message: err.Error()}
}
