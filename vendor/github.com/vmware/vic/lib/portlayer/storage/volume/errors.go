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

package volume

type ErrVolumeInUse struct {
	Msg string
}

func (e *ErrVolumeInUse) Error() string {
	return e.Msg
}

func IsErrVolumeInUse(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrVolumeInUse)

	return ok
}

// VolumeStoreNotFoundError : custom error type for when we fail to find a target volume store
type VolumeStoreNotFoundError struct {
	Msg string
}

func (e VolumeStoreNotFoundError) Error() string {
	return e.Msg
}

// VolumeExistsError : custom error type for when a create operation targets and already occupied ID
type VolumeExistsError struct {
	Msg string
}

func (e VolumeExistsError) Error() string {
	return e.Msg
}
