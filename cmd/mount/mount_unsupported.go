// Build for mount for unsupported platforms to stop go complaining
// about "no buildable Go source files "

// Invert the build constraint: linux freebsd

//go:build !linux && !freebsd
// +build !linux,!freebsd

package mount
