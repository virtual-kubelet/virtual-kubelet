// Copyright 2016 VMware, Inc. All Rights Reserved.
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

package extraconfig

import "errors"

// GuestInfoSink uses the rpcvmx mechanism to update the guestinfo key/value map as
// the datasink for encoding target structures
func GuestInfoSink() (DataSink, error) {
	return GuestInfoSinkWithPrefix("")
}

// GuestInfoSinkWithPrefix adds a prefix to all keys accessed. The key must not have leading
// or trailing separator characters, but may have separators in other positions. The separator
// (either . or /) will be replaced with the appropriate value for the key in question.
func GuestInfoSinkWithPrefix(prefix string) (DataSink, error) {
	return nil, errors.New("Not implemented on Windows")
}
