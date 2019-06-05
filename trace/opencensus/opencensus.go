// Package opencensus implements a github.com/virtual-kubelet/virtual-kubelet/trace.Tracer
// using opencensus as a backend.
//
// Use this by setting `trace.T = Adapter{}`
package opencensus

import (
	"context"
	"fmt"
	"sync"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	octrace "go.opencensus.io/trace"
)

const (
	lDebug = "DEBUG"
	lInfo  = "INFO"
	lWarn  = "WARN"
	lErr   = "ERROR"
	lFatal = "FATAL"
)

// Adapter implements the trace.Tracer interface for OpenCensus
type Adapter struct{}

// StartSpan creates a new span from opencensus using the given name.
func (Adapter) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	ctx, ocs := octrace.StartSpan(ctx, name)
	l := log.G(ctx).WithField("method", name)

	s := &span{s: ocs, l: l}
	ctx = log.WithLogger(ctx, s.Logger())

	return ctx, s
}

type span struct {
	mu sync.Mutex
	s  *octrace.Span
	l  log.Logger
}

func (s *span) End() {
	s.s.End()
}

func (s *span) SetStatus(err error) {
	if !s.s.IsRecordingEvents() {
		return
	}

	var status octrace.Status

	if err == nil {
		status.Code = octrace.StatusCodeOK
		s.s.SetStatus(status)
		return
	}

	switch {
	case errdefs.IsNotFound(err):
		status.Code = octrace.StatusCodeNotFound
	case errdefs.IsInvalidInput(err):
		status.Code = octrace.StatusCodeInvalidArgument
		// TODO: other error types
	default:
		status.Code = octrace.StatusCodeUnknown
	}

	status.Message = err.Error()
	s.s.SetStatus(status)
}

func (s *span) WithField(ctx context.Context, key string, val interface{}) context.Context {
	s.mu.Lock()
	s.l = s.l.WithField(key, val)
	ctx = log.WithLogger(ctx, &logger{s: s.s, l: s.l})
	s.mu.Unlock()

	if s.s.IsRecordingEvents() {
		s.s.AddAttributes(makeAttribute(key, val))
	}

	return ctx
}

func (s *span) WithFields(ctx context.Context, f log.Fields) context.Context {
	s.mu.Lock()
	s.l = s.l.WithFields(f)
	ctx = log.WithLogger(ctx, &logger{s: s.s, l: s.l})
	s.mu.Unlock()

	if s.s.IsRecordingEvents() {
		attrs := make([]octrace.Attribute, 0, len(f))
		for k, v := range f {
			attrs = append(attrs, makeAttribute(k, v))
		}
		s.s.AddAttributes(attrs...)
	}

	return ctx
}

func (s *span) Logger() log.Logger {
	return &logger{s: s.s, l: s.l}
}

type logger struct {
	s *octrace.Span
	l log.Logger
	a []octrace.Attribute
}

func (l *logger) Debug(args ...interface{}) {
	if !l.s.IsRecordingEvents() {
		l.l.Debug(args...)
		return
	}

	msg := fmt.Sprint(args...)
	l.l.Debug(msg)
	l.s.Annotate(withLevel(lDebug, l.a), msg)
}

func (l *logger) Debugf(f string, args ...interface{}) {
	l.l.Debugf(f, args)
	l.s.Annotatef(withLevel(lDebug, l.a), f, args...)
}

func (l *logger) Info(args ...interface{}) {
	if !l.s.IsRecordingEvents() {
		l.l.Info(args...)
		return
	}

	msg := fmt.Sprint(args...)
	l.l.Info(msg)
	l.s.Annotate(withLevel(lInfo, l.a), msg)
}

func (l *logger) Infof(f string, args ...interface{}) {
	l.l.Infof(f, args)
	l.s.Annotatef(withLevel(lInfo, l.a), f, args...)
}

func (l *logger) Warn(args ...interface{}) {
	if !l.s.IsRecordingEvents() {
		l.l.Warn(args...)
		return
	}

	msg := fmt.Sprint(args...)
	l.l.Warn(msg)
	l.s.Annotate(withLevel(lWarn, l.a), msg)
}

func (l *logger) Warnf(f string, args ...interface{}) {
	l.l.Warnf(f, args)
	l.s.Annotatef(withLevel(lWarn, l.a), f, args...)
}

func (l *logger) Error(args ...interface{}) {
	if !l.s.IsRecordingEvents() {
		l.l.Error(args...)
		return
	}

	msg := fmt.Sprint(args...)
	l.l.Error(msg)
	l.s.Annotate(withLevel(lErr, l.a), msg)
}

func (l *logger) Errorf(f string, args ...interface{}) {
	l.l.Errorf(f, args)
	l.s.Annotatef(withLevel(lErr, l.a), f, args...)
}

func (l *logger) Fatal(args ...interface{}) {
	if !l.s.IsRecordingEvents() {
		l.l.Fatal(args...)
		return
	}

	msg := fmt.Sprint(args...)
	l.s.Annotate(withLevel(lFatal, l.a), msg)
	l.l.Fatal(msg)
}

func (l *logger) Fatalf(f string, args ...interface{}) {
	l.s.Annotatef(withLevel(lFatal, l.a), f, args...)
	l.l.Fatalf(f, args)
}

func (l *logger) WithError(err error) log.Logger {
	log := l.l.WithError(err)

	var a []octrace.Attribute
	if l.s.IsRecordingEvents() {
		a = make([]octrace.Attribute, len(l.a), len(l.a)+1)
		copy(a, l.a)
		a = append(l.a, makeAttribute("err", err))
	}

	return &logger{s: l.s, l: log, a: a}
}

func (l *logger) WithField(k string, value interface{}) log.Logger {
	log := l.l.WithField(k, value)

	var a []octrace.Attribute

	if l.s.IsRecordingEvents() {
		a = make([]octrace.Attribute, len(l.a), len(l.a)+1)
		copy(a, l.a)
		a = append(a, makeAttribute(k, value))
	}

	return &logger{s: l.s, a: a, l: log}
}

func (l *logger) WithFields(fields log.Fields) log.Logger {
	log := l.l.WithFields(fields)

	var a []octrace.Attribute
	if l.s.IsRecordingEvents() {
		a = make([]octrace.Attribute, len(l.a), len(l.a)+len(fields))
		copy(a, l.a)
		for k, v := range fields {
			a = append(a, makeAttribute(k, v))
		}
	}

	return &logger{s: l.s, a: a, l: log}
}

func makeAttribute(key string, val interface{}) octrace.Attribute {
	var attr octrace.Attribute

	switch v := val.(type) {
	case string:
		attr = octrace.StringAttribute(key, v)
	case int64:
		attr = octrace.Int64Attribute(key, v)
	case bool:
		attr = octrace.BoolAttribute(key, v)
	case error:
		attr = octrace.StringAttribute(key, v.Error())
	default:
		attr = octrace.StringAttribute(key, fmt.Sprintf("%+v", val))
	}

	return attr
}

func withLevel(l string, attrs []octrace.Attribute) []octrace.Attribute {
	return append(attrs, octrace.StringAttribute("level", l))
}
