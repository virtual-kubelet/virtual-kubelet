package suite

import (
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
)

// Suite
type Suite interface {
	Setup()
	Teardown()
}

// Run is
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

// failOnPanic
func failOnPanic(t *testing.T) {
	r := recover()
	if r != nil {
		t.Errorf("test panicked: %v\n%s", r, debug.Stack())
		t.FailNow()
	}
}
