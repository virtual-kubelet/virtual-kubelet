package suite

import (
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
)

// TestFunc defines the test function in a test case
type TestFunc func(*testing.T)

// FailOnPanicFunc defines the function to run when there is a panic
type FailOnPanicFunc func(*testing.T)

// TestSuite contains methods that defines the lifecycle of a test suite
type TestSuite interface {
	Setup()
	Teardown()
}

// TestSkipper allows providers to skip certain tests
type TestSkipper interface {
	ShouldSkipTest(string) bool
}

// TestPanicker allows providers to implement custom logic when the test suite panics
type TestPanicker interface {
	FailOnPanic(*testing.T)
}

type testCase struct {
	name string
	f    TestFunc
}

// Run runs tests registered in the test suite
func Run(t *testing.T, ts TestSuite) {
	var failOnPanic FailOnPanicFunc
	if tPanicker, ok := ts.(TestPanicker); ok {
		failOnPanic = tPanicker.FailOnPanic
	} else {
		failOnPanic = defaultFailOnPanic
	}

	defer failOnPanic(t)

	ts.Setup()
	defer ts.Teardown()

	// The implementation below is based on https://github.com/stretchr/testify
	testFinder := reflect.TypeOf(ts)
	tests := []testCase{}
	for i := 0; i < testFinder.NumMethod(); i++ {
		method := testFinder.Method(i)

		// Test function name must start with "Test"
		if !strings.HasPrefix(method.Name, "Test") {
			continue
		}

		test := testCase{
			name: method.Name,
			f: func(t *testing.T) {
				if tSkipper, ok := ts.(TestSkipper); ok && tSkipper.ShouldSkipTest(method.Name) {
					t.Skipf("Skipped by provider")
				}
				method.Func.Call([]reflect.Value{reflect.ValueOf(ts), reflect.ValueOf(t)})
			},
		}
		tests = append(tests, test)
	}

	for _, test := range tests {
		t.Run(test.name, test.f)
	}
}

// defaultFailOnPanic recovers from test panicking and mark that test as failed
func defaultFailOnPanic(t *testing.T) {
	if r := recover(); r != nil {
		t.Fatalf("%v\n%s", r, debug.Stack())
	}
}
