package root

import (
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"go.opencensus.io/trace"
)

type TracingExporterOptions struct {
	Tags        map[string]string
	ServiceName string
}

var (
	tracingExporters = make(map[string]TracingExporterInitFunc)
)

// TracingExporterInitFunc is the function that is called to initialize an exporter.
// This is used when registering an exporter and called when a user specifed they want to use the exporter.
type TracingExporterInitFunc func(TracingExporterOptions) (trace.Exporter, error)

// RegisterTracingExporter registers a tracing exporter.
// For a user to select an exporter, it must be registered here.
func RegisterTracingExporter(name string, f TracingExporterInitFunc) {
	tracingExporters[name] = f
}

// GetTracingExporter gets the specified tracing exporter passing in the options to the exporter init function.
// For an exporter to be availbale here it must be registered with `RegisterTracingExporter`.
func GetTracingExporter(name string, opts TracingExporterOptions) (trace.Exporter, error) {
	f, ok := tracingExporters[name]
	if !ok {
		return nil, errdefs.NotFoundf("tracing exporter %q not found", name)
	}
	return f(opts)
}

// AvailableTraceExporters gets the list of registered exporters
func AvailableTraceExporters() []string {
	out := make([]string, 0, len(tracingExporters))
	for k := range tracingExporters {
		out = append(out, k)
	}
	return out
}
