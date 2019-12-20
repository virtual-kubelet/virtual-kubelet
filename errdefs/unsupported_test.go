package errdefs

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

type testingUnsupportedError bool

func (e testingUnsupportedError) Error() string {
	return fmt.Sprintf("%v", bool(e))
}

func (e testingUnsupportedError) Unsupported() bool {
	return bool(e)
}

func TestIsUnsupported(t *testing.T) {
	type testCase struct {
		name         string
		err          error
		xMsg         string
		xUnsupported bool
	}

	for _, c := range []testCase{
		{
			name:         "Unsupportedf",
			err:          Unsupportedf("%s unsupported", "foo"),
			xMsg:         "foo unsupported",
			xUnsupported: true,
		},
		{
			name:         "AsUnsupported",
			err:          AsUnsupported(errors.New("this is a test")),
			xMsg:         "this is a test",
			xUnsupported: true,
		},
		{
			name:         "AsUnsupportedWithNil",
			err:          AsUnsupported(nil),
			xMsg:         "",
			xUnsupported: false,
		},
		{
			name:         "nilError",
			err:          nil,
			xMsg:         "",
			xUnsupported: false,
		},
		{
			name:         "customUnsupportedFalse",
			err:          testingUnsupportedError(false),
			xMsg:         "false",
			xUnsupported: false,
		},
		{
			name:         "customUnsupportedTrue",
			err:          testingUnsupportedError(true),
			xMsg:         "true",
			xUnsupported: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(IsUnsupported(c.err), c.xUnsupported))
			if c.err != nil {
				assert.Check(t, cmp.Equal(c.err.Error(), c.xMsg))
			}
		})
	}
}

func TestUnsupportedCause(t *testing.T) {
	err := errors.New("test")
	e := &unsupportedError{err}
	assert.Check(t, cmp.Equal(e.Cause(), err))
	assert.Check(t, IsUnsupported(errors.Wrap(e, "some details")))
}
