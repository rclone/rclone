// Build for mount for unsupported platforms to stop go complaining
// about "no buildable Go source files "

// +build !linux
// +build !darwin !amd64

package mount2
