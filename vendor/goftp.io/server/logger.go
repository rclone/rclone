// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"fmt"
	"log"
)

// Logger represents an interface to record all ftp information and command
type Logger interface {
	Print(sessionID string, message interface{})
	Printf(sessionID string, format string, v ...interface{})
	PrintCommand(sessionID string, command string, params string)
	PrintResponse(sessionID string, code int, message string)
}

// StdLogger use an instance of this to log in a standard format
type StdLogger struct{}

// Print impelment Logger
func (logger *StdLogger) Print(sessionID string, message interface{}) {
	log.Printf("%s  %s", sessionID, message)
}

// Printf impelment Logger
func (logger *StdLogger) Printf(sessionID string, format string, v ...interface{}) {
	logger.Print(sessionID, fmt.Sprintf(format, v...))
}

// PrintCommand impelment Logger
func (logger *StdLogger) PrintCommand(sessionID string, command string, params string) {
	if command == "PASS" {
		log.Printf("%s > PASS ****", sessionID)
	} else {
		log.Printf("%s > %s %s", sessionID, command, params)
	}
}

// PrintResponse impelment Logger
func (logger *StdLogger) PrintResponse(sessionID string, code int, message string) {
	log.Printf("%s < %d %s", sessionID, code, message)
}

// DiscardLogger represents a silent logger, produces no output
type DiscardLogger struct{}

// Print impelment Logger
func (logger *DiscardLogger) Print(sessionID string, message interface{}) {}

// Printf impelment Logger
func (logger *DiscardLogger) Printf(sessionID string, format string, v ...interface{}) {}

// PrintCommand impelment Logger
func (logger *DiscardLogger) PrintCommand(sessionID string, command string, params string) {}

// PrintResponse impelment Logger
func (logger *DiscardLogger) PrintResponse(sessionID string, code int, message string) {}
