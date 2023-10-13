package nfs

import (
	"fmt"
	"log"
	"os"
)

var (
	Log Logger = &DefaultLogger{}
)

type LogLevel int

const (
	PanicLevel LogLevel = iota
	FatalLevel
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
	TraceLevel

	panicLevelStr string = "[PANIC] "
	fatalLevelStr string = "[FATAL] "
	errorLevelStr string = "[ERROR] "
	warnLevelStr  string = "[WARN] "
	infoLevelStr  string = "[INFO] "
	debugLevelStr string = "[DEBUG] "
	traceLevelStr string = "[TRACE] "
)

type Logger interface {
	SetLevel(level LogLevel)
	GetLevel() LogLevel
	ParseLevel(level string) (LogLevel, error)

	Panic(args ...interface{})
	Fatal(args ...interface{})
	Error(args ...interface{})
	Warn(args ...interface{})
	Info(args ...interface{})
	Debug(args ...interface{})
	Trace(args ...interface{})
	Print(args ...interface{})

	Panicf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Tracef(format string, args ...interface{})
	Printf(format string, args ...interface{})
}

type DefaultLogger struct {
	Level LogLevel
}

func SetLogger(logger Logger) {
	Log = logger
}

func init() {
	if os.Getenv("LOG_LEVEL") != "" {
		if level, err := Log.ParseLevel(os.Getenv("LOG_LEVEL")); err == nil {
			Log.SetLevel(level)
		}
	} else {
		// set default log level to info
		Log.SetLevel(InfoLevel)
	}
}

func (l *DefaultLogger) GetLevel() LogLevel {
	return l.Level
}

func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.Level = level
}

func (l *DefaultLogger) ParseLevel(level string) (LogLevel, error) {
	switch level {
	case "panic":
		return PanicLevel, nil
	case "fatal":
		return FatalLevel, nil
	case "error":
		return ErrorLevel, nil
	case "warn":
		return WarnLevel, nil
	case "info":
		return InfoLevel, nil
	case "debug":
		return DebugLevel, nil
	case "trace":
		return TraceLevel, nil
	}
	var ll LogLevel
	return ll, fmt.Errorf("invalid log level %q", level)
}

func (l *DefaultLogger) Panic(args ...interface{}) {
	if l.Level < PanicLevel {
		return
	}
	args = append([]interface{}{panicLevelStr}, args...)
	log.Print(args...)
}

func (l *DefaultLogger) Panicf(format string, args ...interface{}) {
	if l.Level < PanicLevel {
		return
	}
	log.Printf(panicLevelStr+format, args...)
}

func (l *DefaultLogger) Fatal(args ...interface{}) {
	if l.Level < FatalLevel {
		return
	}
	args = append([]interface{}{fatalLevelStr}, args...)
	log.Print(args...)
}

func (l *DefaultLogger) Fatalf(format string, args ...interface{}) {
	if l.Level < FatalLevel {
		return
	}
	log.Printf(fatalLevelStr+format, args...)
}

func (l *DefaultLogger) Error(args ...interface{}) {
	if l.Level < ErrorLevel {
		return
	}
	args = append([]interface{}{errorLevelStr}, args...)
	log.Print(args...)
}

func (l *DefaultLogger) Errorf(format string, args ...interface{}) {
	if l.Level < ErrorLevel {
		return
	}
	log.Printf(errorLevelStr+format, args...)
}

func (l *DefaultLogger) Warn(args ...interface{}) {
	if l.Level < WarnLevel {
		return
	}
	args = append([]interface{}{warnLevelStr}, args...)
	log.Print(args...)
}

func (l *DefaultLogger) Warnf(format string, args ...interface{}) {
	if l.Level < WarnLevel {
		return
	}
	log.Printf(warnLevelStr+format, args...)
}

func (l *DefaultLogger) Info(args ...interface{}) {
	if l.Level < InfoLevel {
		return
	}
	args = append([]interface{}{infoLevelStr}, args...)
	log.Print(args...)
}

func (l *DefaultLogger) Infof(format string, args ...interface{}) {
	if l.Level < InfoLevel {
		return
	}
	log.Printf(infoLevelStr+format, args...)
}

func (l *DefaultLogger) Debug(args ...interface{}) {
	if l.Level < DebugLevel {
		return
	}
	args = append([]interface{}{debugLevelStr}, args...)
	log.Print(args...)
}

func (l *DefaultLogger) Debugf(format string, args ...interface{}) {
	if l.Level < DebugLevel {
		return
	}
	log.Printf(debugLevelStr+format, args...)
}

func (l *DefaultLogger) Trace(args ...interface{}) {
	if l.Level < TraceLevel {
		return
	}
	args = append([]interface{}{traceLevelStr}, args...)
	log.Print(args...)
}

func (l *DefaultLogger) Tracef(format string, args ...interface{}) {
	if l.Level < TraceLevel {
		return
	}
	log.Printf(traceLevelStr+format, args...)
}

func (l *DefaultLogger) Print(args ...interface{}) {
	log.Print(args...)
}

func (l *DefaultLogger) Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}
