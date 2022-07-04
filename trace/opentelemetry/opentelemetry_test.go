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

package openTelemetry

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

func TestStartSpan(t *testing.T) {
	t.Run("addField", func(t *testing.T) {
		tearDown, p, _ := setupSuite()
		defer tearDown(p)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		a := Adapter{}
		ctx, s := a.StartSpan(ctx, "name")
		s.End()
	})
}

func TestSetStatus(t *testing.T) {
	testCases := []struct {
		description         string
		spanName            string
		inputStatus         error
		expectedCode        codes.Code
		expectedDescription string
	}{
		{
			description:         "error status",
			spanName:            "test",
			inputStatus:         errors.New("fake msg"),
			expectedCode:        codes.Error,
			expectedDescription: "fake msg",
		}, {
			description:         "non-error status",
			spanName:            "test",
			inputStatus:         nil,
			expectedCode:        codes.Ok,
			expectedDescription: "",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.description, func(t *testing.T) {
			tearDown, p, e := setupSuite()
			defer tearDown(p)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctx, ots := otel.Tracer(tt.spanName).Start(ctx, tt.spanName)
			l := log.G(ctx).WithField("method", tt.spanName)

			s := &span{s: ots, l: l}
			s.SetStatus(tt.inputStatus)
			assert.Assert(t, s.s.IsRecording())

			s.End()

			assert.Assert(t, !s.s.IsRecording())
			assert.Assert(t, e.status.Code == tt.expectedCode)
			assert.Assert(t, e.status.Description == tt.expectedDescription)
			s.SetStatus(tt.inputStatus) // should not be panic even if span is ended.
		})
	}
}

func TestWithField(t *testing.T) {
	type field struct {
		key   string
		value interface{}
	}

	testCases := []struct {
		description        string
		spanName           string
		fields             []field
		expectedAttributes []attribute.KeyValue
	}{
		{
			description:        "single field",
			spanName:           "test",
			fields:             []field{{key: "testKey1", value: "value1"}},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}},
		}, {
			description:        "multiple unique fields",
			spanName:           "test",
			fields:             []field{{key: "testKey1", value: "value1"}, {key: "testKey2", value: "value2"}},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}, {Key: "testKey2", Value: attribute.StringValue("value2")}},
		}, {
			description:        "duplicated fields",
			spanName:           "test",
			fields:             []field{{key: "testKey1", value: "value1"}, {key: "testKey1", value: "value2"}},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value2")}},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.description, func(t *testing.T) {
			tearDown, p, e := setupSuite()
			defer tearDown(p)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctx, ots := otel.Tracer(tt.spanName).Start(ctx, tt.spanName)
			l := log.G(ctx).WithField("method", tt.spanName)
			s := &span{s: ots, l: l}

			for _, f := range tt.fields {
				ctx = s.WithField(ctx, f.key, f.value)
			}
			s.End()

			assert.Assert(t, len(e.attributes) == len(tt.expectedAttributes))
			for _, a := range tt.expectedAttributes {
				cmp.Contains(e.attributes, a)
			}
		})
	}
}

func TestWithFields(t *testing.T) {
	testCases := []struct {
		description        string
		spanName           string
		fields             log.Fields
		expectedAttributes []attribute.KeyValue
	}{
		{
			description:        "single field",
			spanName:           "test",
			fields:             log.Fields{"testKey1": "value1"},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}},
		}, {
			description:        "multiple unique fields",
			spanName:           "test",
			fields:             log.Fields{"testKey1": "value1", "testKey2": "value2"},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}, {Key: "testKey2", Value: attribute.StringValue("value2")}},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.description, func(t *testing.T) {
			tearDown, p, e := setupSuite()
			defer tearDown(p)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctx, ots := otel.Tracer(tt.spanName).Start(ctx, tt.spanName)
			l := log.G(ctx).WithField("method", tt.spanName)
			s := &span{s: ots, l: l}

			ctx = s.WithFields(ctx, tt.fields)
			s.End()

			assert.Assert(t, len(e.attributes) == len(tt.expectedAttributes))
			for _, a := range tt.expectedAttributes {
				cmp.Contains(e.attributes, a)
			}
		})
	}
}

func TestLog(t *testing.T) {
	testCases := []struct {
		description        string
		spanName           string
		logLevel           logLevel
		fields             log.Fields
		msg                string
		expectedAttributes []attribute.KeyValue
	}{
		{
			description:        "debug",
			spanName:           "test",
			logLevel:           lDebug,
			fields:             log.Fields{"testKey1": "value1"},
			msg:                "message",
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}},
		}, {
			description:        "info",
			spanName:           "test",
			logLevel:           lInfo,
			fields:             log.Fields{"testKey1": "value1"},
			msg:                "message",
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}},
		}, {
			description:        "warn",
			spanName:           "test",
			logLevel:           lWarn,
			fields:             log.Fields{"testKey1": "value1"},
			msg:                "message",
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}},
		}, {
			description:        "error",
			spanName:           "test",
			logLevel:           lErr,
			fields:             log.Fields{"testKey1": "value1"},
			msg:                "message",
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}},
		}, {
			description:        "fatal",
			spanName:           "test",
			logLevel:           lFatal,
			fields:             log.Fields{"testKey1": "value1"},
			msg:                "message",
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.description, func(t *testing.T) {
			tearDown, p, _ := setupSuite()
			defer tearDown(p)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctx, s := otel.Tracer(tt.spanName).Start(ctx, tt.spanName)
			fl := &fakeLogger{}
			l := logger{s: s, l: fl, a: make([]attribute.KeyValue, 0)}
			switch tt.logLevel {
			case lDebug:
				l.WithFields(tt.fields).Debug(tt.msg)
			case lInfo:
				l.WithFields(tt.fields).Info(tt.msg)
			case lWarn:
				l.WithFields(tt.fields).Warn(tt.msg)
			case lErr:
				l.WithFields(tt.fields).Error(tt.msg)
			case lFatal:
				l.WithFields(tt.fields).Fatal(tt.msg)
			}
			s.End()

			//TODO add assertion with exporter here once logic for recoding log event is added.
			assert.Assert(t, len(fl.a) == len(tt.expectedAttributes))
			for _, a := range tt.expectedAttributes {
				cmp.Contains(fl.a, a)
			}
		})
	}
}

func TestLogf(t *testing.T) {
	testCases := []struct {
		description        string
		spanName           string
		logLevel           logLevel
		msg                string
		fields             log.Fields
		args               []interface{}
		expectedAttributes []attribute.KeyValue
	}{
		{
			description: "debug",
			spanName:    "test",
			logLevel:    lDebug,
			msg:         "k1: %s, k2: %v, k3: %d, k4: %v",
			fields:      map[string]interface{}{"k1": "test", "k2": []string{"test"}, "k3": 1, "k4": []int{1}},
			expectedAttributes: []attribute.KeyValue{
				attribute.String("k1", "test"),
				attribute.StringSlice("k2", []string{"test"}),
				attribute.Int("k3", 1),
				attribute.IntSlice("k4", []int{1}),
			},
		}, {
			description: "info",
			spanName:    "test",
			logLevel:    lInfo,
			msg:         "k1: %d, k2: %v, k3: %f, k4: %v",
			fields:      map[string]interface{}{"k1": int64(3), "k2": []int64{4}, "k3": float64(2), "k4": []float64{4}},
			expectedAttributes: []attribute.KeyValue{
				attribute.Int64("k1", 1),
				attribute.Int64Slice("k2", []int64{2}),
				attribute.Float64("k3", 3),
				attribute.Float64Slice("k2", []float64{4}),
			},
		}, {
			description: "warn",
			spanName:    "test",
			logLevel:    lWarn,
			msg:         "k1: %v, k2: %v",
			fields:      map[string]interface{}{"k1": map[int]int{1: 1}, "k2": num(1)},
			expectedAttributes: []attribute.KeyValue{
				attribute.String("k1", "{1:1}"),
				attribute.Stringer("k2", num(1)),
			},
		}, {
			description: "error",
			spanName:    "test",
			logLevel:    lErr,
			msg:         "k1: %t, k2: %v",
			fields:      map[string]interface{}{"k1": true, "k2": []bool{true}, "k3": errors.New("fake")},
			expectedAttributes: []attribute.KeyValue{
				attribute.Bool("k1", true),
				attribute.BoolSlice("k2", []bool{true}),
				attribute.String("k3", "fake"),
			},
		}, {
			description:        "fatal",
			spanName:           "test",
			logLevel:           lFatal,
			expectedAttributes: []attribute.KeyValue{},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.description, func(t *testing.T) {
			tearDown, p, _ := setupSuite()
			defer tearDown(p)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctx, s := otel.Tracer(tt.spanName).Start(ctx, tt.spanName)
			fl := &fakeLogger{}
			l := logger{s: s, l: fl, a: make([]attribute.KeyValue, 0)}
			switch tt.logLevel {
			case lDebug:
				l.WithFields(tt.fields).Debugf(tt.msg, tt.args)
			case lInfo:
				l.WithFields(tt.fields).Infof(tt.msg, tt.args)
			case lWarn:
				l.WithFields(tt.fields).Warnf(tt.msg, tt.args)
			case lErr:
				l.WithFields(tt.fields).Errorf(tt.msg, tt.args)
			case lFatal:
				l.WithFields(tt.fields).Fatalf(tt.msg, tt.args)
			}
			s.End()

			//TODO add assertion with exporter here once logic for recoding log event is added.
			assert.Assert(t, len(fl.a) == len(tt.expectedAttributes))
			for _, a := range tt.expectedAttributes {
				cmp.Contains(fl.a, a)
			}

			l.Debugf(tt.msg, tt.args) // should not panic even if span is finished
		})
	}
}

func TestLogWithField(t *testing.T) {
	type field struct {
		key   string
		value interface{}
	}

	testCases := []struct {
		description        string
		spanName           string
		fields             []field
		expectedAttributes []attribute.KeyValue
	}{
		{
			description:        "single field",
			spanName:           "test",
			fields:             []field{{key: "testKey1", value: "value1"}},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}},
		}, {
			description:        "multiple unique fields",
			spanName:           "test",
			fields:             []field{{key: "testKey1", value: "value1"}, {key: "testKey2", value: "value2"}},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}, {Key: "testKey2", Value: attribute.StringValue("value2")}},
		}, {
			description:        "duplicated fields",
			spanName:           "test",
			fields:             []field{{key: "testKey1", value: "value1"}, {key: "testKey1", value: "value2"}},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}, {Key: "testKey2", Value: attribute.StringValue("value2")}},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.description, func(t *testing.T) {
			tearDown, p, _ := setupSuite()
			defer tearDown(p)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctx, s := otel.Tracer(tt.spanName).Start(ctx, tt.spanName)
			fl := &fakeLogger{}
			l := logger{s: s, l: fl, a: make([]attribute.KeyValue, 0)}

			for _, f := range tt.fields {
				l.WithField(f.key, f.value).Info("")
			}
			s.End()

			assert.Assert(t, len(fl.a) == len(tt.expectedAttributes))
			for _, a := range tt.expectedAttributes {
				cmp.Contains(fl.a, a)
			}

			l.Debug("") // should not panic even if span is finished

		})
	}
}

func TestLogWithError(t *testing.T) {
	testCases := []struct {
		description        string
		spanName           string
		err                error
		expectedAttributes []attribute.KeyValue
	}{
		{
			description:        "normal",
			spanName:           "test",
			err:                errors.New("fake"),
			expectedAttributes: []attribute.KeyValue{{Key: "err", Value: attribute.StringValue("fake")}},
		}, {
			description:        "nil error",
			spanName:           "test",
			err:                nil,
			expectedAttributes: []attribute.KeyValue{{Key: "err", Value: attribute.StringValue("")}},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.description, func(t *testing.T) {
			tearDown, p, _ := setupSuite()
			defer tearDown(p)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctx, s := otel.Tracer(tt.spanName).Start(ctx, tt.spanName)
			fl := &fakeLogger{}
			l := logger{s: s, l: fl, a: make([]attribute.KeyValue, 0)}

			l.WithError(tt.err).Error("")
			s.End()

			assert.Assert(t, len(fl.a) == len(tt.expectedAttributes))
			for _, a := range tt.expectedAttributes {
				cmp.Contains(fl.a, a)
			}
		})
	}
}

func TestLogWithFields(t *testing.T) {
	testCases := []struct {
		description        string
		spanName           string
		fields             log.Fields
		expectedAttributes []attribute.KeyValue
	}{
		{
			description:        "single field",
			spanName:           "test",
			fields:             log.Fields{"testKey1": "value1"},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}},
		}, {
			description:        "multiple unique fields",
			spanName:           "test",
			fields:             log.Fields{"testKey1": "value1", "testKey2": "value2"},
			expectedAttributes: []attribute.KeyValue{{Key: "testKey1", Value: attribute.StringValue("value1")}, {Key: "testKey2", Value: attribute.StringValue("value2")}},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.description, func(t *testing.T) {
			tearDown, p, _ := setupSuite()
			defer tearDown(p)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctx, s := otel.Tracer(tt.spanName).Start(ctx, tt.spanName)
			fl := &fakeLogger{}
			l := logger{s: s, l: fl, a: make([]attribute.KeyValue, 0)}

			l.WithFields(tt.fields).Debug("")
			s.End()

			assert.Assert(t, len(fl.a) == len(tt.expectedAttributes))
			for _, a := range tt.expectedAttributes {
				cmp.Contains(fl.a, a)
			}
		})
	}
}

func setupSuite() (func(provider *sdktrace.TracerProvider), *sdktrace.TracerProvider, *fakeExporter) {
	r := NewResource("virtual-kubelet", "1.2.3")
	e := &fakeExporter{}
	p := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(e),
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(p)

	// Return a function to teardown the test
	return func(provider *sdktrace.TracerProvider) {
		_ = p.Shutdown(context.Background())
	}, p, e
}

func NewResource(name, version string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(name),
		semconv.ServiceVersionKey.String(version),
	)
}

type fakeExporter struct {
	sync.Mutex
	// attributes describe the aspects of the spans.
	attributes []attribute.KeyValue
	// Links returns all the links the span has to other spans.
	links []sdktrace.Link
	// Events returns all the events that occurred within in the spans
	// lifetime.
	events []sdktrace.Event
	// Status returns the spans status.
	status sdktrace.Status
}

func (f *fakeExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) (err error) {
	f.Lock()
	defer f.Unlock()

	f.attributes = make([]attribute.KeyValue, 0)
	f.links = make([]sdktrace.Link, 0)
	f.events = make([]sdktrace.Event, 0)
	for _, s := range spans {
		f.attributes = append(f.attributes, s.Attributes()...)
		f.links = append(f.links, s.Links()...)
		f.events = append(f.events, s.Events()...)
		f.status = s.Status()
	}
	return
}

func (f *fakeExporter) Shutdown(_ context.Context) (err error) {
	f.attributes = make([]attribute.KeyValue, 0)
	f.links = make([]sdktrace.Link, 0)
	f.events = make([]sdktrace.Event, 0)
	return
}

type fakeLogger struct {
	a []attribute.KeyValue
}

func (*fakeLogger) Debug(...interface{})          {}
func (*fakeLogger) Debugf(string, ...interface{}) {}
func (*fakeLogger) Info(...interface{})           {}
func (*fakeLogger) Infof(string, ...interface{})  {}
func (*fakeLogger) Warn(...interface{})           {}
func (*fakeLogger) Warnf(string, ...interface{})  {}
func (*fakeLogger) Error(...interface{})          {}
func (*fakeLogger) Errorf(string, ...interface{}) {}
func (*fakeLogger) Fatal(...interface{})          {}
func (*fakeLogger) Fatalf(string, ...interface{}) {}

func (l *fakeLogger) WithField(k string, v interface{}) log.Logger {
	l.a = append(l.a, makeAttribute(k, v))
	return l
}
func (l *fakeLogger) WithFields(fs log.Fields) log.Logger {
	for k, v := range fs {
		l.a = append(l.a, makeAttribute(k, v))
	}
	return l
}
func (l *fakeLogger) WithError(err error) log.Logger {
	l.a = append(l.a, makeAttribute("err", err))
	return l
}

type num int

func (i num) String() string {
	return strconv.Itoa(int(i))
}
