package suite

import (
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
)

// TestFunc defines the test function in a test case
type TestFunc func(*testing.T)

// SetUpFunc sets up provider-specific resource in the test suite
type SetUpFunc func() error

// TeardownFunc tears down provider-specific resources from the test suite
type TeardownFunc func() error

// ShouldSkipTestFunc determines whether the test suite should skip certain tests
type ShouldSkipTestFunc func(string) bool

// TestSuite contains methods that defines the lifecycle of a test suite
type TestSuite interface {
	Setup(t *testing.T)
	Teardown()
}

// TestSkipper allows providers to skip certain tests
type TestSkipper interface {
	ShouldSkipTest(string) bool
}

type testCase struct {
	name string
	f    TestFunc
}

// Run runs tests registered in the test suite
func Run(t *testing.T, ts TestSuite) {
	defer failOnPanic(t)

	ts.Setup(t)
	defer ts.Teardown()

	// The implementation below is based on https://github.com/stretchr/testify
	testFinder := reflect.TypeOf(ts)
	tests := []testCase{}
	for i := 0; i < testFinder.NumMethod(); i++ {
		method := testFinder.Method(i)
		if !isValidTestFunc(method) {
			continue
		}

		test := testCase{
			name: method.Name,
			f: func(t *testing.T) {
				defer failOnPanic(t)
				if tSkipper, ok := ts.(TestSkipper); ok && tSkipper.ShouldSkipTest(method.Name) {
					t.Skipf("Skipped due to shouldSkipTest()")
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

// failOnPanic recovers panic occurred in the test suite and marks the test / test suite as failed
func failOnPanic(t *testing.T) {
	if r := recover(); r != nil {
		t.Fatalf("%v\n%s", r, debug.Stack())
	}
}

// isValidTestFunc determines whether or not a given method is a valid test function
func isValidTestFunc(method reflect.Method) bool {
	return strings.HasPrefix(method.Name, "Test") && // Test function name must start with "Test",
		method.Type.NumIn() == 2 && // the number of function input should be 2 (*TestSuite ts and t *testing.T),
		method.Type.In(1) == reflect.TypeOf(&testing.T{}) &&
		method.Type.NumOut() == 0 // and the number of function output should be 0
}
