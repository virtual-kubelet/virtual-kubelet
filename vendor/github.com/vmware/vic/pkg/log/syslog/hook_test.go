// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package syslog

import (
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewSyslogHook(t *testing.T) {
	// error case
	d := &mockDialer{}
	d.On("dial").Return(nil, assert.AnError)
	h, err := newHook(d)
	assert.Nil(t, h)
	assert.Error(t, err)
	assert.EqualError(t, err, assert.AnError.Error())
	d.AssertCalled(t, "dial")
	d.AssertNumberOfCalls(t, "dial", 1)

	// no error
	d = &mockDialer{}
	w := &MockWriter{}
	d.On("dial").Return(w, nil)
	h, err = newHook(d)
	assert.NotNil(t, h)
	assert.NoError(t, err)
	assert.Equal(t, w, h.writer)
	d.AssertCalled(t, "dial")
	d.AssertNumberOfCalls(t, "dial", 1)
}

func TestLevels(t *testing.T) {
	m := &MockWriter{}
	d := &mockDialer{}
	d.On("dial").Return(m, nil)
	h, err := newHook(d)

	assert.NotNil(t, h)
	assert.NoError(t, err)

	m.On("Crit", mock.Anything).Return(nil)
	m.On("Err", mock.Anything).Return(nil)
	m.On("Warning", mock.Anything).Return(nil)
	m.On("Debug", mock.Anything).Return(nil)
	m.On("Info", mock.Anything).Return(nil)

	var tests = []struct {
		entry *logrus.Entry
		f     string
	}{
		{
			entry: &logrus.Entry{Message: "panic", Level: logrus.PanicLevel},
			f:     "Crit",
		},
		{
			entry: &logrus.Entry{Message: "fatal", Level: logrus.FatalLevel},
			f:     "Crit",
		},
		{
			entry: &logrus.Entry{Message: "error", Level: logrus.ErrorLevel},
			f:     "Err",
		},
		{
			entry: &logrus.Entry{Message: "warn", Level: logrus.WarnLevel},
			f:     "Warning",
		},
		{
			entry: &logrus.Entry{Message: "info", Level: logrus.InfoLevel},
			f:     "Info",
		},
		{
			entry: &logrus.Entry{Message: "debug", Level: logrus.DebugLevel},
			f:     "Debug",
		},
	}

	calls := make(map[string]int)
	for _, te := range tests {
		calls[te.f] = 0
	}

	for _, te := range tests {
		assert.NoError(t, h.writeEntry(te.entry))
		calls[te.f]++
		m.AssertCalled(t, te.f, te.entry.Message)
		m.AssertNumberOfCalls(t, te.f, calls[te.f])
	}
}

func TestConnect(t *testing.T) {
	// attempt a connection to a server that
	// does not exist
	h, err := NewHook(
		"tcp",
		"foo:514",
		Info,
		"test",
	)

	assert.NoError(t, err)
	assert.NotNil(t, h)

	h.Fire(&logrus.Entry{
		Message: "foo",
		Level:   logrus.InfoLevel,
	})

	<-time.After(5 * time.Second)
}
