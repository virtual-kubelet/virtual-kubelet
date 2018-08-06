// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

import (
	"fmt"
	"net/http"

	derr "github.com/docker/docker/api/errors"
)

// Used to check status code of derr, which is not a public type
type httpStatusError interface {
	HTTPErrorStatusCode() int
}

// InvalidVolumeError is returned when the user specifies a client directory as a volume.
type InvalidVolumeError struct {
}

func (e InvalidVolumeError) Error() string {
	return fmt.Sprintf("mounting directories as a data volume is not supported.")
}

type VolumeExistError struct {
	Volume string
}

func (e VolumeExistError) Error() string {
	return fmt.Sprintf("A volume named %s already exists. Choose a different volume name.", e.Volume)
}

// InvalidBindError is returned when create/run -v has more params than allowed.
type InvalidBindError struct {
	Volume string
}

func (e InvalidBindError) Error() string {
	return fmt.Sprintf("volume bind input is invalid: -v %s", e.Volume)
}

type ServerNotReadyError struct {
	Name string
}

func (e ServerNotReadyError) Error() string {
	return fmt.Sprintf("Server %s not ready", e.Name)
}

func APINotSupportedMsg(product, method string) error {
	return fmt.Errorf("%s does not yet implement %s", product, method)
}

func NillPortlayerClientError(caller string) error {
	return derr.NewErrorWithStatusCode(fmt.Errorf("%s failed to get a portlayer client", caller), http.StatusInternalServerError)
}

// VolumeJoinNotFoundError returns a 404 docker error for a volume join request.
func VolumeJoinNotFoundError(msg string) error {
	return derr.NewRequestNotFoundError(fmt.Errorf(msg))
}

// VolumeCreateNotFoundError returns a 404 docker error for a volume create request.
func VolumeCreateNotFoundError(msg string) error {
	return derr.NewErrorWithStatusCode(fmt.Errorf("No volume store named (%s) exists", msg), http.StatusInternalServerError)
}

// VolumeNotFoundError returns a 404 docker error for a volume get request.
func VolumeNotFoundError(msg string) error {
	return derr.NewErrorWithStatusCode(fmt.Errorf("No such volume: %s", msg), http.StatusNotFound)
}

// VolumeInternalServerError returns a 500 docker error for a volume-related request.
func VolumeInternalServerError(err error) error {
	return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
}

func ContainerResourceNotFoundError(cid, res string) error {
	return derr.NewRequestNotFoundError(fmt.Errorf("No such %s for container: %s", res, cid))
}

func ResourceNotFoundError(res string) error {
	return derr.NewRequestNotFoundError(fmt.Errorf("No such %s", res))
}

// NotFoundError returns a 404 docker error when a container is not found.
func NotFoundError(msg string) error {
	return derr.NewRequestNotFoundError(fmt.Errorf("No such container: %s", msg))
}

func TaskInspectNotFoundError(msg string) error {
	return derr.NewRequestNotFoundError(fmt.Errorf("No container was found with exec id: %s", msg))
}

func TaskBindPowerError() error {
	return derr.NewRequestNotFoundError(fmt.Errorf("the container has been stopped"))
}

func TaskPoweredOffError(msg string) error {
	return derr.NewRequestNotFoundError(fmt.Errorf("container (%s) has been stopped", msg))
}

func ImageNotFoundError(image, tag string) error {
	return derr.NewRequestNotFoundError(fmt.Errorf("An image does not exist locally with the tag: %s", image))
}

func TagNotFoundError(image, tag string) error {
	return derr.NewRequestNotFoundError(fmt.Errorf("tag does not exist: %s:%s", image, tag))
}

// ResourceLockedError returns a 423 http status
func ResourceLockedError(msg string) error {
	return derr.NewErrorWithStatusCode(fmt.Errorf("Resource locked: %s", msg), http.StatusLocked)
}

// InternalServerError returns a 500 docker error on a portlayer error.
func InternalServerError(msg string) error {
	return derr.NewErrorWithStatusCode(fmt.Errorf("Server error from portlayer: %s", msg), http.StatusInternalServerError)
}

// BadRequestError returns a 400 docker error on a bad request.
func BadRequestError(msg string) error {
	return derr.NewErrorWithStatusCode(fmt.Errorf("Bad request error from portlayer: %s", msg), http.StatusBadRequest)
}

func ConflictError(msg string) error {
	return derr.NewRequestConflictError(fmt.Errorf("Conflict error from portlayer: %s", msg))
}

func PluginNotFoundError(name string) error {
	return derr.NewErrorWithStatusCode(fmt.Errorf("plugin %s not found", name), http.StatusNotFound)
}

func SwarmNotSupportedError() error {
	return derr.NewErrorWithStatusCode(fmt.Errorf("Docker Swarm is not yet supported"), http.StatusNotFound)
}

func StreamFormatNotRecognized() error {
	return derr.NewRequestConflictError(fmt.Errorf("Stream format not recognized"))
}

func ConcurrentAPIError(name, request string) error {
	return derr.NewRequestConflictError(fmt.Errorf("%s request is already in progress for container '%s'.", request, name))
}

// Error type check

func IsNotFoundError(err error) bool {
	// if error was created with the docker error function, check the status code
	if httpErr, ok := err.(httpStatusError); ok {
		if httpErr.HTTPErrorStatusCode() == http.StatusNotFound {
			return true
		}
	}

	return false
}

func IsConflictError(err error) bool {
	// if error was created with the docker error function, check the status code
	if httpErr, ok := err.(httpStatusError); ok {
		if httpErr.HTTPErrorStatusCode() == http.StatusConflict {
			return true
		}
	}

	return false
}

func IsResourceInUse(err error) bool {
	if httpErr, ok := err.(httpStatusError); ok {
		if httpErr.HTTPErrorStatusCode() == http.StatusLocked {
			return true
		}
	}

	return false
}

func IsServerNotReady(err error) bool {
	_, ok := err.(ServerNotReadyError)

	return ok
}

func IsVolumeExistError(err error) bool {
	_, ok := err.(VolumeExistError)

	return ok
}

type DetachError struct{}

func (DetachError) Error() string {
	return "detached from container"
}
