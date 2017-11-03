// Errors for go1.8+

//+build go1.8

package vfs

import "os"

// ECLOSED is returned when a handle is closed twice
var ECLOSED = os.ErrClosed
