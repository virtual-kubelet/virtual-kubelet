package logrus

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
)

func TestImplementsLoggerInterface(t *testing.T) {
	l := FromLogrus(&logrus.Entry{})

	if _, ok := l.(log.Logger); !ok {
		t.Fatal("does not implement log.Logger interface")
	}
}
