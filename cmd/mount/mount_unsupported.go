//go:build !linux && !freebsd
// +build !linux,!freebsd

// Package mount implements a FUSE mounting system for rclone remotes.
//
// Build for mount for unsupported platforms to stop go complaining
// about "no buildable Go source files".
//
// Invert the build constraint: linux freebsd
package mount
