// Accounting and limiting reader
// Windows specific functions.

// +build windows

package fs

// startSignalHandler() is Unix specific and does nothing under windows
// platforms.
func startSignalHandler() {}
