package opencensus

import (
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/trace"
)

func TestTracerImplementsTracer(t *testing.T) {
	// ensure that Adapter implements trace.Tracer
	if tt := trace.Tracer(Adapter{}); tt == nil {
	}
}
