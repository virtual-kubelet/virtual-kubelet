/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package log defines the interfaces used for logging in virtual-kubelet.
// It uses a context.Context to store logger details. Additionally you can set
// the default logger to use by setting log.L. This is used when no logger is
// stored in the passed in context.
package log

import (
	"context"
)

var (
	// G is an alias for GetLogger.
	G = GetLogger

	// L is the default logger. It should be initialized before using `G` or `GetLogger`
	// If L is uninitialized and no logger is available in a provided context, a
	// panic will occur.
	L Logger = nopLogger{}
)

type (
	loggerKey struct{}
)

// Logger is the interface used for logging in virtual-kubelet
//
// virtual-kubelet will access the logger via context using `GetLogger` (or its alias, `G`)
// You can set the default logger to use by setting the `L` variable.
type Logger interface {
	Debug(...interface{})
	Debugf(string, ...interface{})
	Info(...interface{})
	Infof(string, ...interface{})
	Warn(...interface{})
	Warnf(string, ...interface{})
	Error(...interface{})
	Errorf(string, ...interface{})
	Fatal(...interface{})
	Fatalf(string, ...interface{})

	WithField(string, interface{}) Logger
	WithFields(Fields) Logger
	WithError(error) Logger
}

// Fields allows setting multiple fields on a logger at one time.
type Fields map[string]interface{}

// WithLogger returns a new context with the provided logger. Use in
// combination with logger.WithField(s) for great effect.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// GetLogger retrieves the current logger from the context. If no logger is
// available, the default logger is returned.
func GetLogger(ctx context.Context) Logger {
	logger := ctx.Value(loggerKey{})

	if logger == nil {
		if L == nil {
			panic("default logger not initialized")
		}
		return L
	}

	return logger.(Logger)
}
