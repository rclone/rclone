// Build for mount for unsupported platforms to stop go complaining
// about "no buildable Go source files "

// Invert the build constraint: linux,go1.11 darwin,go1.11 freebsd,go1.11
//
// !((linux&&go1.11) || (darwin&&go1.11) || (freebsd&&go1.11))
// == !(linux&&go1.11) && !(darwin&&go1.11) && !(freebsd&&go1.11))
// == (!linux || !go1.11) && (!darwin || go1.11) && (!freebsd || !go1.11))

// +build !linux !go1.11
// +build !darwin !go1.11
// +build !freebsd !go1.11

package mount
