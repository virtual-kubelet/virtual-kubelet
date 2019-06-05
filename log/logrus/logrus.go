// Package logrus implements a github.com/virtual-kubelet/virtual-kubelet/log.Logger using Logrus as a backend
// You can use this by creating a logrus logger and calling `FromLogrus(entry)`.
// If you want this to be the default logger for virtual-kubelet, set `log.L` to the value returned by `FromLogrus`
package logrus

import (
	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// Adapter implements the `log.Logger` interface for logrus
type Adapter struct {
	*logrus.Entry
}

// FromLogrus creates a new `log.Logger` from the provided entry
func FromLogrus(entry *logrus.Entry) log.Logger {
	return &Adapter{entry}
}

// WithField adds a field to the log entry.
func (l *Adapter) WithField(key string, val interface{}) log.Logger {
	return FromLogrus(l.Entry.WithField(key, val))
}

// WithFields adds multiple fields to a log entry.
func (l *Adapter) WithFields(f log.Fields) log.Logger {
	return FromLogrus(l.Entry.WithFields(logrus.Fields(f)))
}

// WithError adds an error to the log entry
func (l *Adapter) WithError(err error) log.Logger {
	return FromLogrus(l.Entry.WithError(err))
}
