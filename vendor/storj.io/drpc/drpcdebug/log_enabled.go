// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build debug
// +build debug

package drpcdebug

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var logger = log.New(os.Stderr, "", 0)

// Log executes the callback for a string to log if built with the debug tag.
func Log(cb func() (who, what, why string)) {
	_, file, line, _ := runtime.Caller(1)
	where := fmt.Sprintf("%s:%d", filepath.Base(file), line)
	who, what, why := cb()
	logger.Output(2, fmt.Sprintf("%24s | %-32s | %-6s | %s",
		where, who, what, why))
}

// this exists to work around a bug in markdown doc generation so that it
// does not generate two entries for Enabled, one set to true and one to false.
const enabled = true
