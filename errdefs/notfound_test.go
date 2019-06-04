package errdefs

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

type testingNotFoundError bool

func (e testingNotFoundError) Error() string {
	return fmt.Sprintf("%v", bool(e))
}

func (e testingNotFoundError) NotFound() bool {
	return bool(e)
}

func TestIsNotFound(t *testing.T) {
	type testCase struct {
		name      string
		err       error
		xMsg      string
		xNotFound bool
	}

	for _, c := range []testCase{
		{
			name:      "NotFoundf",
			err:       NotFoundf("%s not found", "foo"),
			xMsg:      "foo not found",
			xNotFound: true,
		},
		{
			name:      "AsNotFound",
			err:       AsNotFound(errors.New("this is a test")),
			xMsg:      "this is a test",
			xNotFound: true,
		},
		{
			name:      "AsNotFoundWithNil",
			err:       AsNotFound(nil),
			xMsg:      "",
			xNotFound: false,
		},
		{
			name:      "nilError",
			err:       nil,
			xMsg:      "",
			xNotFound: false,
		},
		{
			name:      "customNotFoundFalse",
			err:       testingNotFoundError(false),
			xMsg:      "false",
			xNotFound: false,
		},
		{
			name:      "customNotFoundTrue",
			err:       testingNotFoundError(true),
			xMsg:      "true",
			xNotFound: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(IsNotFound(c.err), c.xNotFound))
			if c.err != nil {
				assert.Check(t, cmp.Equal(c.err.Error(), c.xMsg))
			}
		})
	}
}

func TestNotFoundCause(t *testing.T) {
	err := errors.New("test")
	e := &notFoundError{err}
	assert.Check(t, cmp.Equal(e.Cause(), err))
	assert.Check(t, IsNotFound(errors.Wrap(e, "some details")))
}
