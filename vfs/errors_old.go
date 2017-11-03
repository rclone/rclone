// Errors for pre go1.8

//+build !go1.8

package vfs

import "errors"

// ECLOSED is returned when a handle is closed twice
var ECLOSED = errors.New("file already closed")
