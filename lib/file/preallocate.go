package file

import "errors"

// ErrDiskFull is returned from PreAllocate when it detects disk full
var ErrDiskFull = errors.New("preallocate: file too big for remaining disk space")
