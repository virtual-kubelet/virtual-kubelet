<<<<<<< HEAD:vendor/github.com/sirupsen/logrus/terminal_check_js.go
// +build js
=======
// +build appengine gopherjs
>>>>>>> Makefile: remove -i from go test command fixed in golang/go#27285:vendor/github.com/sirupsen/logrus/terminal_check_appengine.go

package logrus

import (
	"io"
)

func checkIfTerminal(w io.Writer) bool {
	return false
}
