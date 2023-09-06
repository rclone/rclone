// Copyright 2019 Caleb Case

// Package tmpfile provides a cross platform facility for creating temporary
// files that are automatically cleaned up (even in the event of an unexpected
// process exit).
//
// tmpfile provides support for at least Linux, OSX, and Windows. Generally any
// POSIX system that adheres to the semantics of unlink
// (https://pubs.opengroup.org/onlinepubs/9699919799/functions/unlink.html)
// should work. Special handling is provided for other platforms.
package tmpfile
