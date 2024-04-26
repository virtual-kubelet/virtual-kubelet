package slog

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// Ensure log.Logger is fully implemented during compile time.
var _ log.Logger = (*adapter)(nil)

const LevelFatal = slog.Level(12)

type adapter struct {
	inner *slog.Logger
}

func (l *adapter) Debug(args ...interface{}) {
	msg := args[0].(string)
	l.inner.Debug(msg)
}

func (l *adapter) Debugf(format string, args ...interface{}) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Debug(formattedArgs)
}

func (l *adapter) Info(args ...interface{}) {
	msg := args[0].(string)
	l.inner.Info(msg)
}

func (l *adapter) Infof(format string, args ...interface{}) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Info(formattedArgs)
}

func (l *adapter) Warn(args ...interface{}) {
	msg := args[0].(string)
	l.inner.Warn(msg)
}

func (l *adapter) Warnf(format string, args ...interface{}) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Warn(formattedArgs)
}

func (l *adapter) Error(args ...interface{}) {
	msg := args[0].(string)
	l.inner.Error(msg)
}

func (l *adapter) Errorf(format string, args ...interface{}) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Error(formattedArgs)
}

func (l *adapter) Fatal(args ...interface{}) {
	msg := args[0].(string)
	l.inner.Log(context.Background(), LevelFatal, msg)
}

func (l *adapter) Fatalf(format string, args ...interface{}) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Log(context.Background(), LevelFatal, formattedArgs)
}

func (l *adapter) WithField(key string, val interface{}) log.Logger {
	return &adapter{inner: l.inner.With(key, val)}
}

func (l *adapter) WithFields(f log.Fields) log.Logger {
	logger := l.inner
	for k, v := range f {
		logger = logger.With(k, v)
	}
	return &adapter{inner: logger}
}

func (l *adapter) WithError(err error) log.Logger {
	return &adapter{inner: l.inner.With("error", err)}
}
