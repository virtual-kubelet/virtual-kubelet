package suite

import (
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
)

// Suite contains methods for setting up and tearing down a tesing suite
type Suite interface {
	Setup()
	Teardown()
}

// Run runs tests registered in the testing suite
func Run(t *testing.T, s Suite) {
	defer failOnPanic(t)

	s.Setup()
	defer s.Teardown()

	// The implementation below is based on https://github.com/stretchr/testify
	testFinder := reflect.TypeOf(s)
	tests := []testing.InternalTest{}
	for i := 0; i < testFinder.NumMethod(); i++ {
		method := testFinder.Method(i)

		// Test function name must start with "Test"
		// TODO: Allow providers to skip particular tests
		if !strings.HasPrefix(method.Name, "Test") {
			continue
		}

		test := testing.InternalTest{
			Name: method.Name,
			F: func(t *testing.T) {
				defer failOnPanic(t)
				method.Func.Call([]reflect.Value{reflect.ValueOf(s), reflect.ValueOf(t)})
			},
		}
		tests = append(tests, test)
	}

	for _, test := range tests {
		t.Run(test.Name, test.F)
	}
}

// failOnPanic recovers from test panicking and mark that test as failed
func failOnPanic(t *testing.T) {
	// The implementation below is based on https://github.com/stretchr/testify
	if r := recover(); r != nil {
		t.Errorf("test panicked: %v\n%s", r, debug.Stack())
		t.FailNow()
	}
}
