package opencensus

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
