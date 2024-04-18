//go:build !((linux && cgo && cmount) || (darwin && cgo && cmount) || (freebsd && cgo && cmount) || (windows && cmount))

// Package cmount implements a FUSE mounting system for rclone remotes.
//
// Build for cmount for unsupported platforms to stop go complaining
// about "no buildable Go source files".
package cmount
