// Build for ncdu for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || solaris || js
// +build plan9 solaris js

package ncdu
