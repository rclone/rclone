// Build for voltn for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || js

// Package voltn provides an interface to the Voltn file sharing platform.
package voltn
