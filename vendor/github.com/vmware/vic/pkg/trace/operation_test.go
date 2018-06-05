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

package trace

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestContextUnpack(t *testing.T) {
	Logger.Level = logrus.DebugLevel

	cnt := 100
	wg := &sync.WaitGroup{}
	wg.Add(cnt)
	for i := 0; i < cnt; i++ {
		go func(i int) {
			defer wg.Done()
			ctx := NewOperation(context.TODO(), "testmsg")

			// unpack an Operation via the context using it's Values fields
			c := FromContext(ctx, "test")
			c.Infof("test info message %d", i)
		}(i) // fix race in test
	}
	wg.Wait()
}

// If we timeout a child, test a stack is printed of contexts
func TestNestedLogging(t *testing.T) {
	// create a buf to check the log
	buf := new(bytes.Buffer)
	Logger.Out = buf

	root := NewOperation(context.Background(), "root")

	var ctxFunc func(parent Operation, level int) Operation

	levels := 10
	ctxFunc = func(parent Operation, level int) Operation {
		if level == levels {
			return parent
		}

		child, _ := WithDeadline(&parent, time.Time{}, fmt.Sprintf("level %d", level))

		return ctxFunc(child, level+1)
	}

	child := ctxFunc(root, 0)

	// Assert the child has an error and prints a stack.  The parent doesn't
	// see this and should not have an error.  Only cancelation trickles up the
	// stack to the parent.
	if !assert.NoError(t, root.Err()) || !assert.Error(t, child.Err()) {
		return
	}

	// Assert we got a stack trace in the log
	log := buf.String()
	lines := strings.Count(log, "\n")
	t.Log(log)

	// Sample stack
	//
	//        ERRO[0000] op=21598.101: github.com/vmware/vic/pkg/trace.TestNestedLogging: level 9 error: context deadline exceeded
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 9
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 8
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 7
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 6
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 5
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 4
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 3
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 2
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 1
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging.func1:71 level 0
	//                        github.com/vmware/vic/pkg/trace.TestNestedLogging:61 root

	// We arrive at 2 because we have the err line (line 0), then the root
	// (line 11) of where we created the ctx.
	if assert.False(t, lines < levels) {
		t.Logf("exepected at least %d and got %d", levels, lines)
		return
	}
}

// Just checking behavior of the context package
func TestSanity(t *testing.T) {
	Logger.Level = logrus.InfoLevel
	levels := 10

	root, cancel := context.WithDeadline(context.Background(), time.Time{})
	defer cancel()

	var ctxFunc func(parent context.Context, level int) context.Context

	ctxFunc = func(parent context.Context, level int) context.Context {
		if level == levels {
			return parent
		}

		child, cancel := context.WithDeadline(parent, time.Now().Add(time.Hour))
		defer cancel()

		return ctxFunc(child, level+1)
	}

	child := ctxFunc(root, 0)

	if !assert.Error(t, child.Err()) {
		t.FailNow()
	}

	err := root.Err()
	if !assert.Error(t, err) {
		return
	}
}

// MockHook is a testify mock that can be registered as a logrus hook
type MockHook struct {
	mock.Mock
}

// Levels indicates that the mock log hook supports all log levels
func (m *MockHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire records that it has been called and returns an error if configured
func (m *MockHook) Fire(entry *logrus.Entry) error {
	args := m.Called(entry)

	return args.Error(0)
}

// cases defines the set of messages we expect to see and the level we expect to see each at
var cases = map[string]logrus.Level{
	"DebugfMessage": logrus.DebugLevel,
	"DebugMessage":  logrus.DebugLevel,
	"InfofMessage":  logrus.InfoLevel,
	"InfoMessage":   logrus.InfoLevel,
	"WarnfMessage":  logrus.WarnLevel,
	"WarnMessage":   logrus.WarnLevel,
	"ErrorfMessage": logrus.ErrorLevel,
	"ErrorMessage":  logrus.ErrorLevel,
}

// buildMatcher creates a testify MatchedBy function for the supplied operation
func buildMatcher(op Operation, shouldContainOpID bool) func(entry *logrus.Entry) bool {
	return func(entry *logrus.Entry) bool {
		if shouldContainOpID && !strings.Contains(entry.Message, op.id) {
			return false // Log message should have contained the operation id, but did not
		}

		if !shouldContainOpID && strings.Contains(entry.Message, op.id) {
			return false // Log message should not have contained the operation id, but did
		}

		for message, level := range cases {
			if entry.Level == level && strings.Contains(entry.Message, message) {
				return true
			}
		}
		return false
	}
}

// TestLogging demonstrates that log messages are relayed from the Operation to the Logger global
func TestLogging(t *testing.T) {
	defer func(original *logrus.Logger) { Logger = original }(Logger)
	Logger = logrus.New()

	op := NewOperation(context.Background(), "TestOperation")

	m := new(MockHook)
	Logger.Hooks.Add(m)
	Logger.Level = logrus.DebugLevel

	m.On("Fire", mock.MatchedBy(buildMatcher(op, true))).Return(nil)

	op.Debugf("DebugfMessage")
	op.Debug("DebugMessage")
	op.Infof("InfofMessage")
	op.Info("InfoMessage")
	op.Warnf("WarnfMessage")
	op.Warn("WarnMessage")
	op.Errorf("ErrorfMessage")
	op.Error(fmt.Errorf("ErrorMessage"))

	m.AssertExpectations(t)
	m.AssertNumberOfCalls(t, "Fire", 8)
}

// TestLogMuxing verifies that an operation-specific Logger can be configured and that both it and
// the global Logger receive messages when logging methods are called on Operation
func TestLogMuxing(t *testing.T) {
	defer func(original *logrus.Logger) { Logger = original }(Logger)
	Logger = logrus.New()

	op := NewOperation(context.Background(), "TestOperation")

	gm := new(MockHook)
	Logger.Hooks.Add(gm)
	Logger.Level = logrus.DebugLevel

	lm := new(MockHook)
	op.Logger = logrus.New()
	op.Logger.Hooks.Add(lm)
	op.Logger.Level = logrus.DebugLevel

	gm.On("Fire", mock.MatchedBy(buildMatcher(op, true))).Return(nil)
	lm.On("Fire", mock.MatchedBy(buildMatcher(op, false))).Return(nil)

	op.Debugf("DebugfMessage")
	op.Debug("DebugMessage")
	op.Infof("InfofMessage")
	op.Info("InfoMessage")
	op.Warnf("WarnfMessage")
	op.Warn("WarnMessage")
	op.Errorf("ErrorfMessage")
	op.Error(fmt.Errorf("ErrorMessage"))

	gm.AssertExpectations(t)
	gm.AssertNumberOfCalls(t, "Fire", 8)

	lm.AssertExpectations(t)
	lm.AssertNumberOfCalls(t, "Fire", 8)
}

// TestLogIsolation verifies that an operation-specific Loggers are actually operation-specific
func TestLogIsolation(t *testing.T) {
	op1 := NewOperation(context.Background(), "TestOperation")
	op2 := NewOperation(context.Background(), "TestOperation")

	lm1 := new(MockHook)
	op1.Logger = logrus.New()
	op1.Logger.Hooks.Add(lm1)
	op1.Logger.Level = logrus.DebugLevel

	lm2 := new(MockHook)
	op2.Logger = logrus.New()
	op2.Logger.Hooks.Add(lm2)
	op2.Logger.Level = logrus.DebugLevel

	lm1.On("Fire", mock.MatchedBy(buildMatcher(op1, false))).Return(nil)

	op1.Debugf("DebugfMessage")
	op1.Info("InfoMessage")
	op1.Warnf("WarnfMessage")
	op1.Errorf("ErrorfMessage")
	op1.Error(fmt.Errorf("ErrorMessage"))

	lm1.AssertExpectations(t)
	lm1.AssertNumberOfCalls(t, "Fire", 5)

	lm2.AssertExpectations(t)
	lm2.AssertNumberOfCalls(t, "Fire", 0)
}

// TestLogInheritance verifies that an operation-specific Loggers are inherited by children
func TestLogInheritance(t *testing.T) {
	op := NewOperation(context.Background(), "TestOperation")

	lm := new(MockHook)
	op.Logger = logrus.New()
	op.Logger.Hooks.Add(lm)
	op.Logger.Level = logrus.DebugLevel

	c1, _ := WithCancel(&op, "CancelChild")
	c2 := WithValue(&c1, "foo", "bar", "ValueChild")
	c3 := FromOperation(c2, "NormalChild")
	c4 := FromContext(c3, "(Should == c3)")

	lm.On("Fire", mock.MatchedBy(buildMatcher(op, false))).Return(nil)

	op.Debugf("DebugfMessage")
	c1.Infof("InfofMessage")
	c2.Warnf("WarnfMessage")
	c3.Errorf("ErrorfMessage")
	c4.Error("ErrorMessage")

	lm.AssertExpectations(t)
	lm.AssertNumberOfCalls(t, "Fire", 5)
}
