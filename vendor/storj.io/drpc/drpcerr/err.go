// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcerr

// Code returns the error code associated with the error or 0 if none is.
func Code(err error) uint64 {
	for i := 0; i < 100; i++ {
		switch v := err.(type) {
		case interface{ Code() uint64 }:
			return v.Code()
		case interface{ Cause() error }:
			err = v.Cause()
		case interface{ Unwrap() error }:
			err = v.Unwrap()
		case nil:
			return 0
		}
	}
	return 0
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
