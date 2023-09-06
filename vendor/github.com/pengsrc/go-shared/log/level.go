package log

import (
	"fmt"
	"strings"
)

// Level defines log levels.
type Level uint8

// String returns name of the level.
func (l Level) String() string {
	switch l {
	case MuteLevel:
		return "MUTE"
	case FatalLevel:
		return "FATAL"
	case PanicLevel:
		return "PANIC"
	case ErrorLevel:
		return "ERROR"
	case WarnLevel:
		return "WARN"
	case InfoLevel:
		return "INFO"
	case DebugLevel:
		return "DEBUG"
	}
	return ""
}

const (
	// MuteLevel disables the logger.
	MuteLevel Level = iota
	// FatalLevel defines fatal log level.
	FatalLevel
	// PanicLevel defines panic log level.
	PanicLevel
	// ErrorLevel defines error log level.
	ErrorLevel
	// WarnLevel defines warn log level.
	WarnLevel
	// InfoLevel defines info log level.
	InfoLevel
	// DebugLevel defines debug log level.
	DebugLevel
)

// ParseLevel takes a string level and returns the log level constant.
func ParseLevel(level string) (Level, error) {
	switch strings.ToUpper(level) {
	case "FATAL":
		return FatalLevel, nil
	case "PANIC":
		return PanicLevel, nil
	case "ERROR":
		return ErrorLevel, nil
	case "WARN":
		return WarnLevel, nil
	case "INFO":
		return InfoLevel, nil
	case "DEBUG":
		return DebugLevel, nil
	}

	return MuteLevel, fmt.Errorf(`"%q" is not a valid log Level`, level)
}
