package lockdeps

import (
	// TODO(Sargun): Remove in Go1.13
	// This is a dep that `go mod tidy` keeps removing, because it's a transitive dep that's pulled in via a test
	// See: https://github.com/golang/go/issues/29702
	_ "github.com/prometheus/client_golang/prometheus"
	_ "golang.org/x/sys/unix"
)
