// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logrus

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
)

// Configure sets up the logrus logger
func Configure(c *Config, logger *logrus.Logger) error {
	if c.LogLevel != "" {
		lvl, err := logrus.ParseLevel(c.LogLevel)
		if err != nil {
			return errdefs.AsInvalidInput(errors.Wrap(err, "error parsing log level"))
		}

		if logger == nil {
			logger = logrus.StandardLogger()
		}
		logger.SetLevel(lvl)
	}

	return nil
}
