// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package common

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// sdkLogger an interface for logging in the SDK
type sdkLogger interface {
	//LogLevel returns the log level of sdkLogger
	LogLevel() int

	//Log logs v with the provided format if the current log level is loglevel
	Log(logLevel int, format string, v ...interface{}) error
}

// noLogging no logging messages
const noLogging = 0

// infoLogging minimal logging messages
const infoLogging = 1

// debugLogging some logging messages
const debugLogging = 2

// verboseLogging all logging messages
const verboseLogging = 3

// DefaultSDKLogger the default implementation of the sdkLogger
type DefaultSDKLogger struct {
	currentLoggingLevel int
	verboseLogger       *log.Logger
	debugLogger         *log.Logger
	infoLogger          *log.Logger
	nullLogger          *log.Logger
}

// defaultLogger is the defaultLogger in the SDK
var defaultLogger sdkLogger
var loggerLock sync.Mutex
var file *os.File

// initializes the SDK defaultLogger as a defaultLogger
func init() {
	l, _ := NewSDKLogger()
	SetSDKLogger(l)
}

// SetSDKLogger sets the logger used by the sdk
func SetSDKLogger(logger sdkLogger) {
	loggerLock.Lock()
	defaultLogger = logger
	loggerLock.Unlock()
}

// NewSDKLogger creates a defaultSDKLogger
// Debug logging is turned on/off by the presence of the environment variable "OCI_GO_SDK_DEBUG"
// The value of the "OCI_GO_SDK_DEBUG" environment variable controls the logging level.
// "null" outputs no log messages
// "i" or "info" outputs minimal log messages
// "d" or "debug" outputs some logs messages
// "v" or "verbose" outputs all logs messages, including body of requests
func NewSDKLogger() (DefaultSDKLogger, error) {
	logger := DefaultSDKLogger{}

	logger.currentLoggingLevel = noLogging
	logger.verboseLogger = log.New(os.Stderr, "VERBOSE ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	logger.debugLogger = log.New(os.Stderr, "DEBUG ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	logger.infoLogger = log.New(os.Stderr, "INFO ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	logger.nullLogger = log.New(ioutil.Discard, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	configured, isLogEnabled := os.LookupEnv("OCI_GO_SDK_DEBUG")

	// If env variable not present turn logging off
	if !isLogEnabled {
		logger.currentLoggingLevel = noLogging
	} else {
		logOutputModeConfig(logger)

		switch strings.ToLower(configured) {
		case "null":
			logger.currentLoggingLevel = noLogging
			break
		case "i", "info":
			logger.currentLoggingLevel = infoLogging
			break
		case "d", "debug":
			logger.currentLoggingLevel = debugLogging
			break
		//1 here for backwards compatibility
		case "v", "verbose", "1":
			logger.currentLoggingLevel = verboseLogging
			break
		default:
			logger.currentLoggingLevel = infoLogging
		}
		logger.infoLogger.Println("logger level set to: ", logger.currentLoggingLevel)
	}

	return logger, nil
}

func (l DefaultSDKLogger) getLoggerForLevel(logLevel int) *log.Logger {
	if logLevel > l.currentLoggingLevel {
		return l.nullLogger
	}

	switch logLevel {
	case noLogging:
		return l.nullLogger
	case infoLogging:
		return l.infoLogger
	case debugLogging:
		return l.debugLogger
	case verboseLogging:
		return l.verboseLogger
	default:
		return l.nullLogger
	}
}

// Set SDK Log output mode
// Output mode is switched based on environment variable "OCI_GO_SDK_LOG_OUPUT_MODE"
// "file" outputs log to a specific file
// "combine" outputs log to both stderr and specific file
// other unsupported value outputs log to stderr
// output file can be set via environment variable "OCI_GO_SDK_LOG_FILE"
// if this environment variable is not set, a default log file will be created under project root path
func logOutputModeConfig(logger DefaultSDKLogger) {
	logMode, isLogOutputModeEnabled := os.LookupEnv("OCI_GO_SDK_LOG_OUTPUT_MODE")
	if !isLogOutputModeEnabled {
		return
	}
	fileName, isLogFileNameProvided := os.LookupEnv("OCI_GO_SDK_LOG_FILE")
	if !isLogFileNameProvided {
		fileName = fmt.Sprintf("logging_%v%s", time.Now().Unix(), ".log")
	}

	switch strings.ToLower(logMode) {
	case "file", "f":
		file = openLogOutputFile(logger, fileName)
		logger.infoLogger.SetOutput(file)
		logger.debugLogger.SetOutput(file)
		logger.verboseLogger.SetOutput(file)
		break
	case "combine", "c":
		file = openLogOutputFile(logger, fileName)
		wrt := io.MultiWriter(os.Stderr, file)

		logger.infoLogger.SetOutput(wrt)
		logger.debugLogger.SetOutput(wrt)
		logger.verboseLogger.SetOutput(wrt)
		break
	}
}

func openLogOutputFile(logger DefaultSDKLogger, fileName string) *os.File {
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		logger.verboseLogger.Fatal(err)
	}
	return file
}

// CloseLogFile close the logging file and return error
func CloseLogFile() error {
	return file.Close()
}

// LogLevel returns the current debug level
func (l DefaultSDKLogger) LogLevel() int {
	return l.currentLoggingLevel
}

// Log logs v with the provided format if the current log level is loglevel
func (l DefaultSDKLogger) Log(logLevel int, format string, v ...interface{}) error {
	logger := l.getLoggerForLevel(logLevel)
	logger.Output(4, fmt.Sprintf(format, v...))
	return nil
}

// Logln logs v appending a new line at the end
// Deprecated
func Logln(v ...interface{}) {
	defaultLogger.Log(infoLogging, "%v\n", v...)
}

// Logf logs v with the provided format
func Logf(format string, v ...interface{}) {
	defaultLogger.Log(infoLogging, format, v...)
}

// Debugf logs v with the provided format if debug mode is set
func Debugf(format string, v ...interface{}) {
	defaultLogger.Log(debugLogging, format, v...)
}

// Debug  logs v if debug mode is set
func Debug(v ...interface{}) {
	m := fmt.Sprint(v...)
	defaultLogger.Log(debugLogging, "%s", m)
}

// Debugln logs v appending a new line if debug mode is set
func Debugln(v ...interface{}) {
	m := fmt.Sprint(v...)
	defaultLogger.Log(debugLogging, "%s\n", m)
}

// IfDebug executes closure if debug is enabled
func IfDebug(fn func()) {
	if defaultLogger.LogLevel() >= debugLogging {
		fn()
	}
}

// IfInfo executes closure if info is enabled
func IfInfo(fn func()) {
	if defaultLogger.LogLevel() >= infoLogging {
		fn()
	}
}
