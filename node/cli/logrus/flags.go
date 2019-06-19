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
	"github.com/spf13/pflag"
)

// Config is used to configure a logrus logger from CLI flags.
type Config struct {
	LogLevel string
}

// FlagSet creates a new flag set based on the current config
func (c *Config) FlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("logrus", pflag.ContinueOnError)
	flags.StringVar(&c.LogLevel, "log-level", c.LogLevel, `set the log level, e.g. "debug", "info", "warn", "error"`)
	return flags
}
