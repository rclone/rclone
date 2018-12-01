package logger

import (
	"io"

	"github.com/sirupsen/logrus"
)

var _ Logger = Logrus{}
var _ FieldLogger = Logrus{}
var _ Outable = Logrus{}

// Logrus is a Logger implementation backed by sirupsen/logrus
type Logrus struct {
	logrus.FieldLogger
}

// SetOutput will try and set the output of the underlying
// logrus.FieldLogger if it can
func (l Logrus) SetOutput(w io.Writer) {
	if lg, ok := l.FieldLogger.(Outable); ok {
		lg.SetOutput(w)
	}
}

// WithField returns a new Logger with the field added
func (l Logrus) WithField(s string, i interface{}) FieldLogger {
	return Logrus{l.FieldLogger.WithField(s, i)}
}

// WithFields returns a new Logger with the fields added
func (l Logrus) WithFields(m map[string]interface{}) FieldLogger {
	return Logrus{l.FieldLogger.WithFields(m)}
}
