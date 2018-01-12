// Accounting and limiting reader
// Non-unix specific functions.

// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris

package accounting

// startSignalHandler() is Unix specific and does nothing under non-Unix
// platforms.
func startSignalHandler() {}
