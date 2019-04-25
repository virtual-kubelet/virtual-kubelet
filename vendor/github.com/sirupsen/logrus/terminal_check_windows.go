<<<<<<< HEAD:vendor/github.com/sirupsen/logrus/terminal_check_windows.go
// +build !appengine,!js,windows
=======
// +build !appengine,!gopherjs
>>>>>>> Makefile: remove -i from go test command fixed in golang/go#27285:vendor/github.com/sirupsen/logrus/terminal_check_notappengine.go

package logrus

import (
	"io"
	"os"
<<<<<<< HEAD:vendor/github.com/sirupsen/logrus/terminal_check_windows.go
	"syscall"
=======

	"golang.org/x/crypto/ssh/terminal"
>>>>>>> Makefile: remove -i from go test command fixed in golang/go#27285:vendor/github.com/sirupsen/logrus/terminal_check_notappengine.go
)

func checkIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
<<<<<<< HEAD:vendor/github.com/sirupsen/logrus/terminal_check_windows.go
		var mode uint32
		err := syscall.GetConsoleMode(syscall.Handle(v.Fd()), &mode)
		return err == nil
=======
		return terminal.IsTerminal(int(v.Fd()))
>>>>>>> Makefile: remove -i from go test command fixed in golang/go#27285:vendor/github.com/sirupsen/logrus/terminal_check_notappengine.go
	default:
		return false
	}
}
