// Build for tui for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || js || aix

// Package tui provides drawing helpers for rclone's text based user interfaces
package tui
