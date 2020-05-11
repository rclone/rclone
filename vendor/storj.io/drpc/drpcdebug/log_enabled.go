// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build debug

package drpcdebug

import (
	"log"
	"os"
)

var logger = log.New(os.Stderr, "", 0)

// Log executes the callback for a string to log if built with the debug tag.
func Log(cb func() string) {
	logger.Output(2, "\t"+cb())
}
