// Copyright © 2022 The virtual-kubelet authors
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

// Package opentelemetry implements a github.com/virtual-kubelet/virtual-kubelet/trace.Tracer
// using openTelemetry as a backend.
//
// Use this by setting `trace.T = Adapter{}`
//
// For customizing trace provider used in Adapter, set trace provider by
// `otel.SetTracerProvider(*sdktrace.TracerProvider)`. Examples of customize are setting service name,
// use your own exporter (e.g. jaeger, otlp, prometheus, zipkin, and stdout) etc. Do not forget
// to call TracerProvider.Shutdown() when you create your TracerProvider to avoid memory leak.
package opentelemetry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel/codes"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	ot "go.opentelemetry.io/otel/trace"
)

type logLevel string

const (
	lDebug logLevel = "DEBUG"
	lInfo  logLevel = "INFO"
	lWarn  logLevel = "WARN"
	lErr   logLevel = "ERROR"
	lFatal logLevel = "FATAL"
)

// Adapter implements the trace.Tracer interface for openTelemetry
type Adapter struct{}

// StartSpan creates a new span from openTelemetry using the given name.
func (Adapter) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	ctx, ots := otel.Tracer(name).Start(ctx, name)
	l := log.G(ctx).WithField("method", name)

	s := &span{s: ots, l: l}
	ctx = log.WithLogger(ctx, s.Logger())

	return ctx, s
}

type span struct {
	mu sync.Mutex
	s  ot.Span
	l  log.Logger
}

func (s *span) End() {
	s.s.End()
}

func (s *span) SetStatus(err error) {
	if !s.s.IsRecording() {
		return
	}

	if err == nil {
		s.s.SetStatus(codes.Ok, "")
	} else {
		s.s.SetStatus(codes.Error, err.Error())
	}
}

func (s *span) WithField(ctx context.Context, key string, val interface{}) context.Context {
	s.mu.Lock()
	s.l = s.l.WithField(key, val)
	ctx = log.WithLogger(ctx, &logger{s: s.s, l: s.l})
	s.mu.Unlock()

	if s.s.IsRecording() {
		s.s.SetAttributes(makeAttribute(key, val))
	}

	return ctx
}

func (s *span) WithFields(ctx context.Context, f log.Fields) context.Context {
	s.mu.Lock()
	s.l = s.l.WithFields(f)
	ctx = log.WithLogger(ctx, &logger{s: s.s, l: s.l})
	s.mu.Unlock()

	if s.s.IsRecording() {
		attrs := make([]attribute.KeyValue, 0, len(f))
		for k, v := range f {
			attrs = append(attrs, makeAttribute(k, v))
		}
		s.s.SetAttributes(attrs...)
	}
	return ctx
}

func (s *span) Logger() log.Logger {
	return &logger{s: s.s, l: s.l}
}

type logger struct {
	s ot.Span
	l log.Logger
	a []attribute.KeyValue
}

func (l *logger) Debug(args ...interface{}) {
	l.logEvent(lDebug, args...)
}

func (l *logger) Debugf(f string, args ...interface{}) {
	l.logEventf(lDebug, f, args...)
}

func (l *logger) Info(args ...interface{}) {
	l.logEvent(lInfo, args...)
}

func (l *logger) Infof(f string, args ...interface{}) {
	l.logEventf(lInfo, f, args...)
}

func (l *logger) Warn(args ...interface{}) {
	l.logEvent(lWarn, args...)
}

func (l *logger) Warnf(f string, args ...interface{}) {
	l.logEventf(lWarn, f, args...)
}

func (l *logger) Error(args ...interface{}) {
	l.logEvent(lErr, args...)
}

func (l *logger) Errorf(f string, args ...interface{}) {
	l.logEventf(lErr, f, args...)
}

func (l *logger) Fatal(args ...interface{}) {
	l.logEvent(lFatal, args...)
}

func (l *logger) Fatalf(f string, args ...interface{}) {
	l.logEventf(lFatal, f, args...)
}

func (l *logger) logEvent(ll logLevel, args ...interface{}) {
	msg := fmt.Sprint(args...)
	switch ll {
	case lDebug:
		l.l.Debug(msg)
	case lInfo:
		l.l.Info(msg)
	case lWarn:
		l.l.Warn(msg)
	case lErr:
		l.l.Error(msg)
	case lFatal:
		l.l.Fatal(msg)
	}

	if !l.s.IsRecording() {
		return
	}
	l.s.AddEvent(msg, ot.WithTimestamp(time.Now()))
}

func (l *logger) logEventf(ll logLevel, f string, args ...interface{}) {
	switch ll {
	case lDebug:
		l.l.Debugf(f, args...)
	case lInfo:
		l.l.Infof(f, args...)
	case lWarn:
		l.l.Warnf(f, args...)
	case lErr:
		l.l.Errorf(f, args...)
	case lFatal:
		l.l.Fatalf(f, args...)
	}

	if !l.s.IsRecording() {
		return
	}
	msg := fmt.Sprintf(f, args...)
	l.s.AddEvent(msg, ot.WithTimestamp(time.Now()))
}

func (l *logger) WithError(err error) log.Logger {
	return l.WithField("err", err)
}

func (l *logger) WithField(k string, value interface{}) log.Logger {
	var attrs []attribute.KeyValue
	if l.s.IsRecording() {
		attrs = make([]attribute.KeyValue, len(l.a)+1)
		copy(attrs, l.a)
		attrs[len(attrs)-1] = makeAttribute(k, value)
	}
	return &logger{s: l.s, a: attrs, l: l.l.WithField(k, value)}
}

func (l *logger) WithFields(fields log.Fields) log.Logger {
	var attrs []attribute.KeyValue
	if l.s.IsRecording() {
		attrs = make([]attribute.KeyValue, len(l.a), len(l.a)+len(fields))
		copy(attrs, l.a)
		for k, v := range fields {
			attrs = append(attrs, makeAttribute(k, v))
		}
	}
	return &logger{s: l.s, a: attrs, l: l.l.WithFields(fields)}
}

func makeAttribute(key string, val interface{}) (attr attribute.KeyValue) {
	switch v := val.(type) {
	case string:
		return attribute.String(key, v)
	// case []string:
	// 	return attribute.StringSlice(key, v)
	case fmt.Stringer:
		return attribute.Stringer(key, v)
	case int:
		return attribute.Int(key, v)
	// case []int:
	// 	return attribute.IntSlice(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	// case []float64:
	// 	return attribute.Float64Slice(key, v)
	// case []int64:
	// 	return attribute.Int64Slice(key, v)
	case bool:
		return attribute.Bool(key, v)
	// case []bool:
	// 	return attribute.BoolSlice(key, v)
	case error:
		if v == nil {
			return attribute.String(key, "")
		}
		return attribute.String(key, v.Error())
	default:
		return attribute.String(key, fmt.Sprintf("%+v", val))
	}
}
