package errdefs

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

type testingInvalidInputError bool

func (e testingInvalidInputError) Error() string {
	return fmt.Sprintf("%v", bool(e))
}

func (e testingInvalidInputError) InvalidInput() bool {
	return bool(e)
}

func TestIsInvalidInput(t *testing.T) {
	type testCase struct {
		name          string
		err           error
		xMsg          string
		xInvalidInput bool
	}

	for _, c := range []testCase{
		{
			name:          "InvalidInputf",
			err:           InvalidInputf("%s not found", "foo"),
			xMsg:          "foo not found",
			xInvalidInput: true,
		},
		{
			name:          "AsInvalidInput",
			err:           AsInvalidInput(errors.New("this is a test")),
			xMsg:          "this is a test",
			xInvalidInput: true,
		},
		{
			name:          "AsInvalidInputWithNil",
			err:           AsInvalidInput(nil),
			xMsg:          "",
			xInvalidInput: false,
		},
		{
			name:          "nilError",
			err:           nil,
			xMsg:          "",
			xInvalidInput: false,
		},
		{
			name:          "customInvalidInputFalse",
			err:           testingInvalidInputError(false),
			xMsg:          "false",
			xInvalidInput: false,
		},
		{
			name:          "customInvalidInputTrue",
			err:           testingInvalidInputError(true),
			xMsg:          "true",
			xInvalidInput: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(IsInvalidInput(c.err), c.xInvalidInput))
			if c.err != nil {
				assert.Check(t, cmp.Equal(c.err.Error(), c.xMsg))
			}
		})
	}
}

func TestInvalidInputCause(t *testing.T) {
	err := errors.New("test")
	e := &invalidInputError{err}
	assert.Check(t, cmp.Equal(e.Cause(), err))
	assert.Check(t, IsInvalidInput(errors.Wrap(e, "some details")))
}
