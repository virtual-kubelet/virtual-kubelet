package status

import (
	"net/http"

	"github.com/cpuguy83/strongerrors"
)

// HTTPCode takes an error and returns the HTTP status code for the given error
// If a match is found then the second return argument will be true, otherwise it will be false.
// nolint: gocyclo
func HTTPCode(err error) (int, bool) {
	switch {
	case strongerrors.IsNotFound(err):
		return http.StatusNotFound, true
	case strongerrors.IsInvalidArgument(err):
		return http.StatusBadRequest, true
	case strongerrors.IsConflict(err):
		return http.StatusConflict, true
	case strongerrors.IsUnauthenticated(err), strongerrors.IsForbidden(err):
		return http.StatusForbidden, true
	case strongerrors.IsUnauthorized(err):
		return http.StatusUnauthorized, true
	case strongerrors.IsUnavailable(err):
		return http.StatusServiceUnavailable, true
	case strongerrors.IsForbidden(err):
		return http.StatusForbidden, true
	case strongerrors.IsAlreadyExists(err), strongerrors.IsNotModified(err):
		return http.StatusNotModified, true
	case strongerrors.IsNotImplemented(err):
		return http.StatusNotImplemented, true
	case strongerrors.IsSystem(err) || strongerrors.IsUnknown(err) || strongerrors.IsDataLoss(err) || strongerrors.IsExhausted(err):
		return http.StatusInternalServerError, true
	default:
		return http.StatusInternalServerError, false
	}
}
