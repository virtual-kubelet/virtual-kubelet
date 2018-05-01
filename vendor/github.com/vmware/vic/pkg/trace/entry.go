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

package trace

import (
	"github.com/Sirupsen/logrus"
)

// Entry is like logrus.Entry, but for Operation. Functionality is added as needed.
type Entry struct {
	// Contains all the fields set by the user.
	Data logrus.Fields

	// A reference to the operation-local logger the entry was constructed for.
	local *logrus.Logger
}

func (o *Operation) WithFields(fields logrus.Fields) *Entry {
	entry := Entry{
		Data:  fields,
		local: o.Logger,
	}

	return &entry
}

func (e *Entry) Debug(args ...interface{}) {
	Logger.WithFields(e.Data).Debug(args...)

	if e.local != nil {
		e.local.WithFields(e.Data).Debug(args...)
	}
}
