// Build for cmount for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build !((linux && cgo && cmount) || (darwin && cgo && cmount) || (freebsd && cgo && cmount) || (windows && cmount))
// +build !linux !cgo !cmount
// +build !darwin !cgo !cmount
// +build !freebsd !cgo !cmount
// +build !windows !cmount

package cmount
