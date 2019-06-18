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

package root

import (
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"go.opencensus.io/trace"
)

func TestGetTracingExporter(t *testing.T) {
	defer delete(tracingExporters, "mock")

	mockExporterFn := func(_ TracingExporterOptions) (trace.Exporter, error) {
		return nil, nil
	}

	_, err := GetTracingExporter("notexist", TracingExporterOptions{})
	if !errdefs.IsNotFound(err) {
		t.Fatalf("expected not found error, got: %v", err)
	}

	RegisterTracingExporter("mock", mockExporterFn)

	if _, err := GetTracingExporter("mock", TracingExporterOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestAvailableExporters(t *testing.T) {
	defer delete(tracingExporters, "mock")

	mockExporterFn := func(_ TracingExporterOptions) (trace.Exporter, error) {
		return nil, nil
	}
	RegisterTracingExporter("mock", mockExporterFn)

	for _, e := range AvailableTraceExporters() {
		if e == "mock" {
			return
		}
	}

	t.Fatal("could not find mock exporter in list of registered exporters")
}
