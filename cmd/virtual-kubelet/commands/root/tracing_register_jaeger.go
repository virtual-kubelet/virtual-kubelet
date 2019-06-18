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

// +build !no_jaeger_exporter

package root

import (
	"errors"
	"os"

	"go.opencensus.io/exporter/jaeger"
	"go.opencensus.io/trace"
)

func init() {
	RegisterTracingExporter("jaeger", NewJaegerExporter)
}

// NewJaegerExporter creates a new opencensus tracing exporter.
func NewJaegerExporter(opts TracingExporterOptions) (trace.Exporter, error) {
	jOpts := jaeger.Options{
		Endpoint:      os.Getenv("JAEGER_ENDPOINT"),
		AgentEndpoint: os.Getenv("JAEGER_AGENT_ENDPOINT"),
		Username:      os.Getenv("JAEGER_USER"),
		Password:      os.Getenv("JAEGER_PASSWORD"),
		Process: jaeger.Process{
			ServiceName: opts.ServiceName,
		},
	}

	if jOpts.Endpoint == "" && jOpts.AgentEndpoint == "" {
		return nil, errors.New("Must specify either JAEGER_ENDPOINT or JAEGER_AGENT_ENDPOINT")
	}

	for k, v := range opts.Tags {
		jOpts.Process.Tags = append(jOpts.Process.Tags, jaeger.StringTag(k, v))
	}
	return jaeger.NewExporter(jOpts)
}
