package log

type nopLogger struct{}

func (nopLogger) Debug(...interface{})          {}
func (nopLogger) Debugf(string, ...interface{}) {}
func (nopLogger) Info(...interface{})           {}
func (nopLogger) Infof(string, ...interface{})  {}
func (nopLogger) Warn(...interface{})           {}
func (nopLogger) Warnf(string, ...interface{})  {}
func (nopLogger) Error(...interface{})          {}
func (nopLogger) Errorf(string, ...interface{}) {}
func (nopLogger) Fatal(...interface{})          {}
func (nopLogger) Fatalf(string, ...interface{}) {}

func (l nopLogger) WithField(string, interface{}) Logger { return l }
func (l nopLogger) WithFields(Fields) Logger             { return l }
func (l nopLogger) WithError(error) Logger               { return l }
