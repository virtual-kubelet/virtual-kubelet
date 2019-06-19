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
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/node/cli/opts"
	"go.opencensus.io/trace"
	"go.opencensus.io/zpages"
)

// ExporterInitFunc is the function that is called to initialize an exporter.
// This is used when registering an exporter and called when a user specifed they want to use the exporter.
type ExporterInitFunc func(*Config) (trace.Exporter, error)

func (c *Config) getExporter(name string) (trace.Exporter, error) {
	init, ok := c.AvailableExporters[name]
	if !ok {
		return nil, errdefs.NotFoundf("exporter not found: %s", name)
	}
	return init(c)
}

var reservedTagNames = map[string]bool{
	"operatingSystem": true,
	"provider":        true,
	"nodeName":        true,
}

// Configure sets up opecensus from the passed in config
func Configure(ctx context.Context, c *Config, o *opts.Opts) error {
	for k := range c.Tags {
		if reservedTagNames[k] {
			return errdefs.InvalidInputf("invalid trace tag %q, must not use a reserved tag key", k)
		}
	}

	if c.Tags == nil {
		c.Tags = make(map[string]string, 3)
	}

	c.Tags["operatingSystem"] = o.OperatingSystem
	c.Tags["provider"] = o.Provider
	c.Tags["nodeName"] = o.NodeName

	for _, e := range c.Exporters {
		if e == "zpages" {
			if c.ZpagesAddr == "" {
				log.G(ctx).Warn("Zpages trace exporter requested but listen address was not set, sipping")
			}
			setupZpages(ctx, c.ZpagesAddr)
			continue
		}

		exporter, err := c.getExporter(e)
		if err != nil {
			return err
		}

		trace.RegisterExporter(exporter)
	}

	if len(c.Exporters) == 0 {
		return nil
	}

	var s trace.Sampler
	switch strings.ToLower(c.SampleRate) {
	case "":
	case "always":
		s = trace.AlwaysSample()
	case "never":
		s = trace.NeverSample()
	default:
		rate, err := strconv.Atoi(c.SampleRate)
		if err != nil {
			return errdefs.AsInvalidInput(errors.Wrap(err, "unsupported trace sample rate"))
		}
		if rate < 0 || rate > 100 {
			return errdefs.AsInvalidInput(errors.Wrap(err, "trace sample rate must be between 0 and 100"))
		}
		s = trace.ProbabilitySampler(float64(rate) / 100)
	}

	if s != nil {
		trace.ApplyConfig(
			trace.Config{
				DefaultSampler: s,
			},
		)
	}

	return nil
}

func setupZpages(ctx context.Context, addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.G(ctx).WithError(err).Error("Could not bind to specified zpages addr: %s", addr)
		return
	}

	mux := http.NewServeMux()
	zpages.Handle(mux, "/debug")

	go func() {
		// This should never terminate, if it does, it will always terminate with an error
		e := http.Serve(listener, mux)
		if e == http.ErrServerClosed {
			return
		}
		log.G(ctx).WithError(e).Error("Zpages server exited")
	}()
}
