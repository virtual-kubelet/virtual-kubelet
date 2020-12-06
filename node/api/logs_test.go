package api

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

// func parseLogOptions(q url.Values) (opts ContainerLogOpts, err error)
func TestParseLogOptions(t *testing.T) {
	// tailLines
	// follow
	// limitBytes
	// previous
	// sinceSeconds
	// sinceTime
	// timestamps
	sinceTime, _ := time.Parse(time.RFC3339, "2020-03-20T21:07:34Z")
	fmt.Printf("%+v\n", sinceTime)
	testCases := []struct {
		Values  url.Values
		Failure bool
		Result  ContainerLogOpts
	}{
		{
			Values:  url.Values{},
			Failure: false,
			Result:  ContainerLogOpts{},
		},
		{
			Values: url.Values{
				"follow":       {"true"},
				"limitBytes":   {"123"},
				"previous":     {"true"},
				"sinceSeconds": {"10"},
				"tailLines":    {"99"},
				"timestamps":   {"true"},
			},
			Failure: false,
			Result: ContainerLogOpts{
				Follow:       true,
				LimitBytes:   123,
				Previous:     true,
				SinceSeconds: 10,
				Tail:         99,
				Timestamps:   true,
			},
		},
		{
			Values: url.Values{
				"sinceSeconds": {"10"},
				"sinceTime":    {"2020-03-20T21:07:34Z"},
			},
			Failure: true,
		},
		{
			Values: url.Values{
				"sinceTime": {"2020-03-20T21:07:34Z"},
			},
			Failure: false,
			Result: ContainerLogOpts{
				SinceTime: sinceTime,
			},
		},
		{
			Values: url.Values{
				"tailLines": {"-1"},
			},
			Failure: true,
		},
		{
			Values: url.Values{
				"limitBytes": {"0"},
			},
			Failure: true,
		},
		{
			Values: url.Values{
				"sinceSeconds": {"-10"},
			},
			Failure: true,
		},
	}
	// follow=true&limitBytes=1&previous=true&sinceSeconds=1&sinceTime=2020-03-20T21%3A07%3A34Z&tailLines=1&timestamps=true
	for i, tc := range testCases {
		msg := fmt.Sprintf("test case #%d %+v failed", i+1, tc)
		result, err := parseLogOptions(tc.Values)
		if tc.Failure {
			assert.Check(t, is.ErrorContains(err, ""), msg)
		} else {
			assert.NilError(t, err, msg)
			assert.Check(t, is.Equal(result, tc.Result), msg)
		}
	}
}
