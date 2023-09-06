// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !debug
// +build !debug

package drpcdebug

// Log executes the callback for a string to log if built with the debug tag.
func Log(cb func() (who, what, why string)) {}

// this exists to work around a bug in markdown doc generation so that it
// does not generate two entries for Enabled, one set to true and one to false.
const enabled = false
