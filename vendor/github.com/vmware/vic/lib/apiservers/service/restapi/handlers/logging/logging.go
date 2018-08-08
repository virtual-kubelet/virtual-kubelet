// Copyright 2017-2018 VMware, Inc. All Rights Reserved.
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

// Package logging encapsulates logging functionality used by API handlers.
package logging

import (
	"github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/install/vchlog"
	viclog "github.com/vmware/vic/pkg/log"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

// SetUpLogger leverages the vchlog package to configure the supplied operation with a VCHLogger which writes to the
// datastore. It is only works for use during VCH creation, but support for other mutation operations should be added.
func SetUpLogger(op *trace.Operation) *vchlog.VCHLogger {
	log := vchlog.New()

	op.Logger = logrus.New()
	op.Logger.Out = log.GetPipe()
	op.Logger.Level = logrus.DebugLevel
	op.Logger.Formatter = viclog.NewTextFormatter()

	op.Logger.Infof("Starting API-based VCH Creation. Version: %q", version.GetBuild().ShortVersion())

	go log.Run()

	return log
}
