// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !debug

package drpcdebug

// Log executes the callback for a string to log if built with the debug tag.
func Log(cb func() string) {}
