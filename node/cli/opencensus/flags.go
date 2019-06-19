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

package opencensus

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

// Config is used to configured a tracer.
type Config struct {
	SampleRate         string
	ServiceName        string
	Tags               map[string]string
	Exporters          []string
	AvailableExporters map[string]ExporterInitFunc
	ZpagesAddr         string
}

func FromEnv() *Config {
	return &Config{ZpagesAddr: os.Getenv("ZPAGES_ADDR")}
}

// FlagSet creates a new flag set based on the current config
func (c *Config) FlagSet() *pflag.FlagSet {
	if len(c.AvailableExporters) == 0 {
		return nil
	}

	flags := pflag.NewFlagSet("opencensus", pflag.ContinueOnError)

	exporters := make([]string, 0, len(c.AvailableExporters))
	for e := range c.AvailableExporters {
		exporters = append(exporters, e)
	}

	flags.StringSliceVar(&c.Exporters, "trace-exporter", c.Exporters, fmt.Sprintf("sets the tracing exporter to use, available exporters: %s", exporters))
	flags.StringVar(&c.ServiceName, "trace-service-name", c.ServiceName, "sets the name of the service used to register with the trace exporter")
	flags.StringToStringVar(&c.Tags, "trace-tag", c.Tags, "add tags to include with traces in key=value form")
	flags.StringVar(&c.SampleRate, "trace-sample-rate", c.SampleRate, "set probability of tracing samples")
	flags.StringVar(&c.ZpagesAddr, "trace-zpages-addr", c.ZpagesAddr, "set the listen address to use for zpages")

	return flags
}
