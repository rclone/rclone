// Build for mount for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build !linux && (!darwin || !amd64)
// +build !linux
// +build !darwin !amd64

package mount2
