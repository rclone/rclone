// Build for mount for unsupported platforms to stop go complaining
// about "no buildable Go source files "

// Invert the build constraint: linux,go1.13 darwin,go1.13 freebsd,go1.13
//
// !((linux&&go1.13) || (darwin&&go1.13) || (freebsd&&go1.13))
// == !(linux&&go1.13) && !(darwin&&go1.13) && !(freebsd&&go1.13))
// == (!linux || !go1.13) && (!darwin || go1.13) && (!freebsd || !go1.13))

// +build !linux !go1.13
// +build !darwin !go1.13
// +build !freebsd !go1.13

package mount
