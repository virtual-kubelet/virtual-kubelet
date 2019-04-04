package status

import (
	"github.com/cpuguy83/strongerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FromGRPC returns an error class from the provided GPRC status
// If the status is nil or OK, this will return nil
// nolint: gocyclo
func FromGRPC(s *status.Status) error {
	if s == nil || s.Code() == codes.OK {
		return nil
	}

	switch s.Code() {
	case codes.InvalidArgument:
		return strongerrors.InvalidArgument(s.Err())
	case codes.NotFound:
		return strongerrors.NotFound(s.Err())
	case codes.Unimplemented:
		return strongerrors.NotImplemented(s.Err())
	case codes.DeadlineExceeded:
		return strongerrors.Deadline(s.Err())
	case codes.Canceled:
		return strongerrors.Cancelled(s.Err())
	case codes.AlreadyExists:
		return strongerrors.AlreadyExists(s.Err())
	case codes.PermissionDenied:
		return strongerrors.Unauthorized(s.Err())
	case codes.Unauthenticated:
		return strongerrors.Unauthenticated(s.Err())
	// TODO(cpuguy83): consider more granular errors for these cases
	case codes.FailedPrecondition, codes.Aborted, codes.Unavailable, codes.OutOfRange:
		return strongerrors.Conflict(s.Err())
	case codes.ResourceExhausted:
		return strongerrors.Exhausted(s.Err())
	case codes.DataLoss:
		return strongerrors.DataLoss(s.Err())
	default:
		return strongerrors.Unknown(s.Err())
	}
}

// ToGRPC takes the passed in error and converts it to a GRPC status error
// If the passed in error is already a gprc status error, then it is returned unmodified
// If the passed in error is nil, then a nil error is returned.
// nolint: gocyclo
func ToGRPC(err error) error {
	if _, ok := status.FromError(err); ok {
		return err
	}

	switch {
	case strongerrors.IsNotFound(err):
		return status.Error(codes.NotFound, err.Error())
	case strongerrors.IsConflict(err), strongerrors.IsNotModified(err):
		return status.Error(codes.FailedPrecondition, err.Error())
	case strongerrors.IsInvalidArgument(err):
		return status.Error(codes.InvalidArgument, err.Error())
	case strongerrors.IsAlreadyExists(err):
		return status.Error(codes.AlreadyExists, err.Error())
	case strongerrors.IsCancelled(err):
		return status.Error(codes.Canceled, err.Error())
	case strongerrors.IsDeadline(err):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case strongerrors.IsUnauthorized(err):
		return status.Error(codes.PermissionDenied, err.Error())
	case strongerrors.IsUnauthenticated(err):
		return status.Error(codes.Unauthenticated, err.Error())
	case strongerrors.IsForbidden(err), strongerrors.IsNotImplemented(err):
		return status.Error(codes.Unimplemented, err.Error())
	case strongerrors.IsExhausted(err):
		return status.Error(codes.ResourceExhausted, err.Error())
	case strongerrors.IsDataLoss(err):
		return status.Error(codes.DataLoss, err.Error())
	case strongerrors.IsSystem(err):
		return status.Error(codes.Internal, err.Error())
	case strongerrors.IsUnavailable(err):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Unknown, err.Error())
	}
}
