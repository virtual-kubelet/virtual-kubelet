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

// Package trace abstracts virtual-kubelet's tracing capabilities into a set of
// interfaces.
// While this does allow consumers to use whatever tracing library they want,
// the primary goal is to share logging data between the configured logger and
// tracing spans instead of duplicating calls.
package trace

import (
	"context"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// Tracer is the interface used for creating a tracing span
type Tracer interface {
	// StartSpan starts a new span. The span details are embedded into the returned
	// context
	StartSpan(context.Context, string) (context.Context, Span)
}

var (
	// T is the Tracer to use this should be initialized before starting up
	// virtual-kubelet
	T Tracer = nopTracer{}
)

type tracerKey struct{}

// WithTracer sets the Tracer which will be used by `StartSpan` in the context.
func WithTracer(ctx context.Context, t Tracer) context.Context {
	return context.WithValue(ctx, tracerKey{}, t)
}

// StartSpan starts a span from the configured default tracer
func StartSpan(ctx context.Context, name string) (context.Context, Span) {
	t := ctx.Value(tracerKey{})

	var tracer Tracer
	if t != nil {
		tracer = t.(Tracer)
	} else {
		tracer = T
	}

	ctx, span := tracer.StartSpan(ctx, name)
	ctx = log.WithLogger(ctx, span.Logger())
	return ctx, span
}

// Span encapsulates a tracing event
type Span interface {
	End()

	// SetStatus sets the final status of the span.
	// errors passed to this should use interfaces defined in
	// github.com/virtual-kubelet/virtual-kubelet/errdefs
	//
	// If the error is nil, the span should be considered successful.
	SetStatus(err error)

	// WithField and WithFields adds attributes to an entire span
	//
	// This interface is a bit weird, but allows us to manage loggers in the context
	// It is expected that implementations set `log.WithLogger` so the logger stored
	// in the context is updated with the new fields.
	WithField(context.Context, string, interface{}) context.Context
	WithFields(context.Context, log.Fields) context.Context

	// Logger is used to log individual entries.
	// Calls to functions like `WithField` and `WithFields` on the logger should
	// not affect the rest of the span but rather individual entries.
	Logger() log.Logger
}
