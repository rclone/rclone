package log

import (
	"context"
)

// ContextFreeLogger is a logger that doesn't take context.
type ContextFreeLogger struct {
	Logger *Logger

	ctx context.Context
}

// Fatal logs a message with severity FATAL followed by a call to os.Exit(1).
func (l *ContextFreeLogger) Fatal(v ...interface{}) {
	l.Logger.event(l.ctx, FatalLevel).write("%v", v...)
}

// Panic logs a message with severity PANIC followed by a call to panic().
func (l *ContextFreeLogger) Panic(v ...interface{}) {
	l.Logger.event(l.ctx, PanicLevel).write("%v", v...)
}

// Error logs a message with severity ERROR.
func (l *ContextFreeLogger) Error(v ...interface{}) {
	l.Logger.event(l.ctx, ErrorLevel).write("%v", v...)
}

// Warn logs a message with severity WARN.
func (l *ContextFreeLogger) Warn(v ...interface{}) {
	l.Logger.event(l.ctx, WarnLevel).write("%v", v...)
}

// Info logs a message with severity INFO.
func (l *ContextFreeLogger) Info(v ...interface{}) {
	l.Logger.event(l.ctx, InfoLevel).write("%v", v...)
}

// Debug logs a message with severity DEBUG.
func (l *ContextFreeLogger) Debug(v ...interface{}) {
	l.Logger.event(l.ctx, DebugLevel).write("%v", v...)
}

// Fatalf logs a message with severity FATAL in format followed by a call to
// os.Exit(1).
func (l *ContextFreeLogger) Fatalf(format string, v ...interface{}) {
	l.Logger.event(l.ctx, FatalLevel).write(format, v...)
}

// Panicf logs a message with severity PANIC in format followed by a call to
// panic().
func (l *ContextFreeLogger) Panicf(format string, v ...interface{}) {
	l.Logger.event(l.ctx, PanicLevel).write(format, v...)
}

// Errorf logs a message with severity ERROR in format.
func (l *ContextFreeLogger) Errorf(format string, v ...interface{}) {
	l.Logger.event(l.ctx, ErrorLevel).write(format, v...)
}

// Warnf logs a message with severity WARN in format.
func (l *ContextFreeLogger) Warnf(format string, v ...interface{}) {
	l.Logger.event(l.ctx, WarnLevel).write(format, v...)
}

// Infof logs a message with severity INFO in format.
func (l *ContextFreeLogger) Infof(format string, v ...interface{}) {
	l.Logger.event(l.ctx, InfoLevel).write(format, v...)
}

// Debugf logs a message with severity DEBUG in format.
func (l *ContextFreeLogger) Debugf(format string, v ...interface{}) {
	l.Logger.event(l.ctx, DebugLevel).write(format, v...)
}

// NewContextFreeLogger creates a new context free logger for given logger.
func NewContextFreeLogger(l *Logger) *ContextFreeLogger {
	return &ContextFreeLogger{Logger: l, ctx: context.Background()}
}
