package log

import (
	"context"
)

// Fatal logs a message with severity FATAL followed by a call to os.Exit(1).
func Fatal(ctx context.Context, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, FatalLevel).write("%v", v...)
	}
}

// Panic logs a message with severity PANIC followed by a call to panic().
func Panic(ctx context.Context, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, PanicLevel).write("%v", v...)
	}
}

// Error logs a message with severity ERROR.
func Error(ctx context.Context, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, ErrorLevel).write("%v", v...)
	}
}

// Warn logs a message with severity WARN.
func Warn(ctx context.Context, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, WarnLevel).write("%v", v...)
	}
}

// Info logs a message with severity INFO.
func Info(ctx context.Context, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, InfoLevel).write("%v", v...)
	}
}

// Debug logs a message with severity DEBUG.
func Debug(ctx context.Context, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, DebugLevel).write("%v", v...)
	}
}

// Fatalf logs a message with severity FATAL in format followed by a call to
// os.Exit(1).
func Fatalf(ctx context.Context, format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, FatalLevel).write(format, v...)
	}
}

// Panicf logs a message with severity PANIC in format followed by a call to
// panic().
func Panicf(ctx context.Context, format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, PanicLevel).write(format, v...)
	}
}

// Errorf logs a message with severity ERROR in format.
func Errorf(ctx context.Context, format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, ErrorLevel).write(format, v...)
	}
}

// Warnf logs a message with severity WARN in format.
func Warnf(ctx context.Context, format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, WarnLevel).write(format, v...)
	}
}

// Infof logs a message with severity INFO in format.
func Infof(ctx context.Context, format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, InfoLevel).write(format, v...)
	}
}

// Debugf logs a message with severity DEBUG in format.
func Debugf(ctx context.Context, format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.event(ctx, DebugLevel).write(format, v...)
	}
}

// FatalEvent returns a log event with severity FATAL.
func FatalEvent(ctx context.Context) *Event {
	if globalLogger != nil {
		return globalLogger.event(ctx, FatalLevel)
	}
	return nil
}

// PanicEvent returns a log event with severity PANIC.
func PanicEvent(ctx context.Context) *Event {
	if globalLogger != nil {
		return globalLogger.event(ctx, PanicLevel)
	}
	return nil
}

// ErrorEvent returns a log event with severity ERROR.
func ErrorEvent(ctx context.Context) *Event {
	if globalLogger != nil {
		return globalLogger.event(ctx, ErrorLevel)
	}
	return nil
}

// WarnEvent returns a log event with severity WARN.
func WarnEvent(ctx context.Context) *Event {
	if globalLogger != nil {
		return globalLogger.event(ctx, WarnLevel)
	}
	return nil
}

// InfoEvent returns a log event with severity INFO.
func InfoEvent(ctx context.Context) *Event {
	if globalLogger != nil {
		return globalLogger.event(ctx, InfoLevel)
	}
	return nil
}

// DebugEvent returns a log event with severity DEBUG.
func DebugEvent(ctx context.Context) *Event {
	if globalLogger != nil {
		return globalLogger.event(ctx, DebugLevel)
	}
	return nil
}

// SetGlobalLogger sets a logger as global logger.
func SetGlobalLogger(l *Logger) {
	globalLogger = l
}

// GlobalLogger returns the global logger.
func GlobalLogger() *Logger {
	return globalLogger
}

var globalLogger *Logger
