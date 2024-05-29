// Accounting and limiting reader
// Non-unix specific functions.

//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package accounting

// startSignalHandler() is Unix specific and does nothing under non-Unix
// platforms.
func (tb *tokenBucket) startSignalHandler() {}
