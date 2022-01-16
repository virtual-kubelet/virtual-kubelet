package logrusr

import (
	"encoding/json"
	"strings"

	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

// According to the specification of the Logger interface calling the InfoLogger
// directly on the logger should be the same as calling them on V(0). Since
// logrus level 0 is PanicLevel and Infolevel doesn't start until V(4) we use
// this constant to be able to calculate what V(n) values should mean.
const (
	logrusDiffToInfo = 4
)

type logrusr struct {
	name             []string
	level            int
	logger           logrus.FieldLogger
	defaultFormatter func(interface{}) string
}

// NewLogger will return a new logr.Logger from a logrus.FieldLogger.
func NewLogger(l logrus.FieldLogger, name ...string) logr.Logger {
	return NewLoggerWithFormatter(l, nil, name...)
}

// NewLoggerWithFormatter will return a new logr.Logger from a
// logrus.FieldLogger that uses provided function to format complex data types.
func NewLoggerWithFormatter(l logrus.FieldLogger, formatter func(interface{}) string, name ...string) logr.Logger {
	return &logrusr{
		name:             name,
		level:            0,
		logger:           l,
		defaultFormatter: formatter,
	}
}

// Enabled is a part of the InfoLogger interface. It will return true if the
// logrus.Logger has a level set to logrus.InfoLevel or higher (Debug/Trace).
// According to the documentation, level V(0) should be equivalent as calling
// Info() directly on the logger. To ensure this the constant `logrusDiffToInfo`
// will be added to all passed values so that V(0) creates a logger with level
// logrus.InfoLevel and V(2) would create a logger with level logrus.TraceLevel.
// This menas that if logrus is  set to logrus.InfoLevel or **higher** this
// method will return true, otherwise false.
func (l *logrusr) Enabled() bool {
	var log *logrus.Logger

	switch t := l.logger.(type) {
	case *logrus.Logger:
		log = t

	case *logrus.Entry:
		log = t.Logger
	}

	// logrus.InfoLevel has value 4 so if the level on the logger is set to 0 we
	// should only be seen as enabled if the logrus logger has a severity of
	// info or higher.
	return int(log.GetLevel())-logrusDiffToInfo >= l.level
}

// V is a part of the Logger interface. Calling the method will change the
// global log severity for the logr implementation.
func (l *logrusr) V(level int) logr.InfoLogger {
	l.level = level

	return l
}

// WithValues is a part of the Logger interface. This is equivalent to
// logrus WithFields() but takes a list of even arguments (key/value pairs)
// instead of a map as input. If an odd number of arguments are sent all values
// will be discarded.
func (l *logrusr) WithValues(keysAndValues ...interface{}) logr.Logger {
	l.logger = l.logger.WithFields(
		listToLogrusFields(l.defaultFormatter, keysAndValues...),
	)

	return l
}

// WithName is a part of the Logger interface. This will set the key "logger" as
// a logrus field to identify the instance.
func (l *logrusr) WithName(name string) logr.Logger {
	l.name = append(l.name, name)

	l.logger = l.logger.WithFields(logrus.Fields{
		"logger": strings.Join(l.name, "."),
	})

	return l
}

// Info logs info messages if the logger is enabled, that is if the level on the
// logger is set to logrus.InfoLevel or less.
func (l *logrusr) Info(msg string, keysAndValues ...interface{}) {
	if !l.Enabled() {
		return
	}

	l.logger.
		WithFields(listToLogrusFields(l.defaultFormatter, keysAndValues...)).
		Info(msg)
}

// Error logs error messages. Since the log will be written with `Error` level
// it won't show if the severity of the underlying logrus logger is less than
// Error.
func (l *logrusr) Error(err error, msg string, keysAndValues ...interface{}) {
	l.logger.
		WithFields(listToLogrusFields(l.defaultFormatter, keysAndValues...)).
		WithError(err).
		Error(msg)
}

// listToLogrusFields converts a list of arbitrary length to key/value paris.
func listToLogrusFields(formatter func(interface{}) string, keysAndValues ...interface{}) logrus.Fields {
	var f = logrus.Fields{}

	// Skip all fields if it's not an even lengthed list.
	if len(keysAndValues)%2 != 0 {
		return f
	}

	for i := 0; i < len(keysAndValues); i += 2 {
		k, v := keysAndValues[i], keysAndValues[i+1]

		if s, ok := k.(string); ok {
			// Try to avoid marshaling known types.
			switch vVal := v.(type) {
			case int, int8, int16, int32, int64,
				uint, uint8, uint16, uint32, uint64,
				float32, float64, complex64, complex128,
				string, bool:
				f[s] = vVal

			case []byte:
				f[s] = string(vVal)

			default:
				if formatter != nil {
					f[s] = formatter(v)
				} else {
					j, _ := json.Marshal(vVal)
					f[s] = string(j)
				}
			}
		}
	}

	return f
}
