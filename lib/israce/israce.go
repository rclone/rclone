//go:build race

// Package israce reports if the Go race detector is enabled.
//
// From https://stackoverflow.com/questions/44944959/how-can-i-check-if-the-race-detector-is-enabled-at-runtime
package israce

// Enabled reports if the race detector is enabled.
const Enabled = true
