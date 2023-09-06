// Package log implements a std log compatible logging system that draws some inspiration from the
// Python standard library [logging module](https://docs.python.org/3/library/logging.html). It
// supports multiple handlers, log levels, zero-allocation, scopes, custom formatting, and
// environment and runtime configuration.
//
// When not used to replace std log, the import should use the package name "analog" as in:
//
//	import analog "github.com/anacrolix/log".
package log
