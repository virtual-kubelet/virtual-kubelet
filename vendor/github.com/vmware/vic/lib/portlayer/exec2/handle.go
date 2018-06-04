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

package exec2

/* A Handle should be completely opaque */
type Handle interface{}

type HandleFactory interface {
	createHandle(cID ID) Handle
	refreshHandle(oldHandle Handle) Handle
}

type BasicHandleFactory struct {
}

func (h *BasicHandleFactory) createHandle(cid ID) Handle {
	newPc := &PendingCommit{}
	newPc.ContainerID = cid
	return newPc
}

// Basic handle resolver just passes back the handle passed in
func (h *BasicHandleFactory) refreshHandle(oldHandle Handle) Handle {
	return oldHandle
}
