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

package main

import (
	"testing"

	opencensuscli "github.com/virtual-kubelet/virtual-kubelet/node/cli/opencensus"
	"go.opencensus.io/trace"
	"gotest.tools/assert"
)

func TestAvailableExporters(t *testing.T) {
	defer delete(tracingExporters, "mock")

	var found bool
	mockExporterFn := func(_ *opencensuscli.Config) (trace.Exporter, error) {
		found = true
		return nil, nil
	}
	RegisterTracingExporter("mock", mockExporterFn)

	for e, f := range AvailableTraceExporters() {
		if e == "mock" {
			_, _ = f(nil)
			assert.Assert(t, found)
			return
		}
	}

	t.Fatal("could not find mock exporter in list of registered exporters")
}
