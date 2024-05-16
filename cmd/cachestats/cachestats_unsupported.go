// Build for cache for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || js

// Package cachestats provides the cachestats command.
package cachestats
