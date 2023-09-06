// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcerr

import "unsafe"

const (
	// Unimplemented is the code used by the generated unimplemented
	// servers when returning errors.
	Unimplemented = 12
)

// Code returns the error code associated with the error or 0 if none is.
func Code(err error) uint64 {
	for i := 0; i < 100; i++ {
		prev := err
		switch v := err.(type) { //nolint: errorlint // this is a custom unwrap loop
		case interface{ Code() uint64 }:
			return v.Code()
		case interface{ Cause() error }:
			err = v.Cause()
		case interface{ Unwrap() error }:
			err = v.Unwrap()
		default:
			return 0
		}
		// short-circuit any trivial cycles
		if shallowEqual(err, prev) {
			return 0
		}
	}
	return 0
}

// shallowEqual returns true if the two errors are equal without comparing
// their values. It may return false even if the errors are equal, but if
// returns true, then the errors are equal.
//
//nolint:gosec
func shallowEqual(x, y error) bool {
	return *(*[2]uintptr)(unsafe.Pointer(&x)) == *(*[2]uintptr)(unsafe.Pointer(&y))
}

// WithCode associates the code with the error if it is non nil and the code
// is non-zero.
func WithCode(err error, code uint64) error {
	if err == nil || code == 0 {
		return err
	}
	return &codeErr{err: err, code: code}
}

type codeErr struct {
	err  error
	code uint64
}

func (c *codeErr) Error() string { return c.err.Error() }
func (c *codeErr) Unwrap() error { return c.err }
func (c *codeErr) Cause() error  { return c.err }
func (c *codeErr) Code() uint64  { return c.code }
