package client

import (
	"errors"
	"fmt"
)

// ErrConnectionFailed is an error raised when the connection between the client and the server failed.
var ErrConnectionFailed = errors.New("Cannot connect to the Hyper.sh server.")

// imageNotFoundError implements an error returned when an image is not in the docker host.
type imageNotFoundError struct {
	imageID string
}

// Error returns a string representation of an imageNotFoundError
func (i imageNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such image: %s", i.imageID)
}

// IsErrImageNotFound returns true if the error is caused
// when an image is not found in the docker host.
func IsErrImageNotFound(err error) bool {
	_, ok := err.(imageNotFoundError)
	return ok
}

// containerNotFoundError implements an error returned when a container is not in the docker host.
type containerNotFoundError struct {
	containerID string
}

// Error returns a string representation of a containerNotFoundError
func (e containerNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such container: %s", e.containerID)
}

// IsErrContainerNotFound returns true if the error is caused
// when a container is not found in the docker host.
func IsErrContainerNotFound(err error) bool {
	_, ok := err.(containerNotFoundError)
	return ok
}

// networkNotFoundError implements an error returned when a network is not in the docker host.
type networkNotFoundError struct {
	networkID string
}

// Error returns a string representation of a networkNotFoundError
func (e networkNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such network: %s", e.networkID)
}

// IsErrNetworkNotFound returns true if the error is caused
// when a network is not found in the docker host.
func IsErrNetworkNotFound(err error) bool {
	_, ok := err.(networkNotFoundError)
	return ok
}

// snapshotNotFoundError implements an error returned when a volume is not in the docker host.
type snapshotNotFoundError struct {
	snapshotID string
}

// Error returns a string representation of an networkNotFoundError
func (e snapshotNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such snapshot: %s", e.snapshotID)
}

// volumeNotFoundError implements an error returned when a volume is not in the docker host.
type volumeNotFoundError struct {
	volumeID string
}

// Error returns a string representation of a networkNotFoundError
func (e volumeNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such volume: %s", e.volumeID)
}

// IsErrVolumeNotFound returns true if the error is caused
// when a volume is not found in the docker host.
func IsErrVolumeNotFound(err error) bool {
	_, ok := err.(volumeNotFoundError)
	return ok
}

// serviceNotFoundError implements an error returned when a service is not in the docker host.
type serviceNotFoundError struct {
	serviceID string
}

// Error returns a string representation of a networkNotFoundError
func (e serviceNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such service: %s", e.serviceID)
}

// IsErrVolumeNotFound returns true if the error is caused
// when a volume is not found in the docker host.
func IsErrServiceNotFound(err error) bool {
	_, ok := err.(serviceNotFoundError)
	return ok
}

// cronNotFoundError implements an error returned when a cron is not in the docker host.
type cronNotFoundError struct {
	cronID string
}

// Error returns a string representation of a networkNotFoundError
func (e cronNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such cron job: %s", e.cronID)
}

// IsErrVolumeNotFound returns true if the error is caused
// when a volume is not found in the docker host.
func IsErrCronNotFound(err error) bool {
	_, ok := err.(cronNotFoundError)
	return ok
}

// funcNotFoundError implements an error returned when a func is not in the docker host.
type funcNotFoundError struct {
	name string
}

// Error returns a string representation of a funcNotFoundError
func (e funcNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such function: %s", e.name)
}

// funcCallNotFoundError implements an error returned when a func call is not in the docker host.
type funcCallNotFoundError struct {
	id string
}

// Error returns a string representation of a funcNotFoundError
func (e funcCallNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such call id: %s", e.id)
}

// IsErrFuncNotFound returns true if the error is caused
// when a func is not found in the docker host.
func IsErrFuncNotFound(err error) bool {
	_, ok := err.(funcNotFoundError)
	return ok
}

// unauthorizedError represents an authorization error in a remote registry.
type unauthorizedError struct {
	cause error
}

// Error returns a string representation of an unauthorizedError
func (u unauthorizedError) Error() string {
	return u.cause.Error()
}

// IsErrUnauthorized returns true if the error is caused
// when a remote registry authentication fails
func IsErrUnauthorized(err error) bool {
	_, ok := err.(unauthorizedError)
	return ok
}
