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
	"errors"
	"sync"
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	octrace "go.opencensus.io/trace"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

// ensure that Adapter implements trace.Tracer
var _ trace.Tracer = (*Adapter)(nil)

type fakeExporter struct {
	sync.Mutex
	spans []*octrace.SpanData
}

func (f *fakeExporter) ExportSpan(s *octrace.SpanData) {
	f.Lock()
	defer f.Unlock()
	f.spans = append(f.spans, s)
}

func TestOpencensus(t *testing.T) {
	t.Run("addField", setupTest(testAddField))
}

func setupTest(f func(t *testing.T, exporter *fakeExporter, l *logger, span *octrace.Span)) func(t *testing.T) {
	return func(t *testing.T) {
		fe := &fakeExporter{}
		octrace.RegisterExporter(fe)
		defer octrace.UnregisterExporter(fe)
		ctx := context.Background()
		_, span := octrace.StartSpan(ctx, t.Name(), octrace.WithSampler(octrace.AlwaysSample()))
		l := &logger{
			s: span,
			l: log.L,
			a: []octrace.Attribute{},
		}
		f(t, fe, l, span)
	}
}

func testAddField(t *testing.T, exporter *fakeExporter, l *logger, span *octrace.Span) {
	assert.Assert(t, l.s.IsRecordingEvents())
	tmpErr := errors.New("test")
	assert.Assert(t, l.s.IsRecordingEvents())
	l.WithField("key", "value").
		WithError(tmpErr).
		WithFields(map[string]interface{}{"test1": "value1", "test2": "value2"}).
		Info()
	span.End()

	assert.Assert(t,
		is.DeepEqual(exporter.spans[0].Annotations[0].Attributes,
			map[string]interface{}{
				"key":   "value",
				"test1": "value1",
				"test2": "value2",
				"err":   "test",
				"level": "INFO",
			}))
}
