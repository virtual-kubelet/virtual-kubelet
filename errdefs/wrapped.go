package errdefs

// Causal is an error interface for errors which have wrapped another error
// in a non-opaque way.
//
// This pattern is used by github.com/pkg/errors
type causal interface {
	Cause() error
	error
}
