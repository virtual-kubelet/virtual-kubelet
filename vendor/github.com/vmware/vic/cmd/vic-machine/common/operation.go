// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package common

import (
	"context"

	"github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/pkg/trace"
)

func NewOperation(clic *cli.Context, debug *int) trace.Operation {
	op := trace.NewOperation(context.Background(), clic.App.Name)
	op.Logger = logrus.New()
	op.Logger.Out = clic.App.Writer

	if debug != nil && *debug > 0 {
		logrus.SetLevel(logrus.DebugLevel)
		trace.Logger.Level = logrus.DebugLevel
		op.Logger.Level = logrus.DebugLevel
	}

	return op
}
