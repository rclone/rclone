// Build for iclouddrive for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || solaris

// Package iclouddrive implements the iCloud Drive backend
package iclouddrive