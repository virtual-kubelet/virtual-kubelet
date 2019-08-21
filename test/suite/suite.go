package suite

import (
	"reflect"
	"strings"
	"testing"
)

// TestingSuite
type Suite interface {
	Setup()
	Teardown()
}

// Run is
func Run(t *testing.T, s Suite) {
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
				method.Func.Call([]reflect.Value{reflect.ValueOf(s)})
			},
		}
		tests = append(tests, test)
	}

	for _, test := range tests {
		t.Run(test.Name, test.F)
	}
}
