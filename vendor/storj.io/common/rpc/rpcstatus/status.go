// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package rpcstatus contains status code definitions for rpc.
package rpcstatus

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"

	"storj.io/common/internal/grpchook"
	"storj.io/drpc/drpcerr"
)

// StatusCode is an enumeration of rpc status codes.
type StatusCode uint64

// These constants are all the rpc error codes. It is important that
// their numerical values do not change.
const (
	Unknown StatusCode = iota
	OK
	Canceled
	InvalidArgument
	DeadlineExceeded
	NotFound
	AlreadyExists
	PermissionDenied
	ResourceExhausted
	FailedPrecondition
	Aborted
	OutOfRange
	Unimplemented
	Internal
	Unavailable
	DataLoss
	Unauthenticated
)

// Code returns the status code associated with the error.
func Code(err error) StatusCode {
	// special case: if the error is context canceled or deadline exceeded, the code
	// must be those. additionally, grpc returns OK for a nil error, so we will, too.
	switch err {
	case nil:
		return OK
	case context.Canceled:
		return Canceled
	case context.DeadlineExceeded:
		return DeadlineExceeded
	default:
		if code := StatusCode(drpcerr.Code(err)); code != Unknown {
			return code
		}

		// If we have grpc attached try to get grpc code if possible.
		if grpchook.HookedConvertToStatusCode != nil {
			if code, ok := grpchook.HookedConvertToStatusCode(err); ok {
				return StatusCode(code)
			}
		}

		return Unknown
	}
}

// Wrap wraps the error with the provided status code.
func Wrap(code StatusCode, err error) error {
	if err == nil {
		return nil
	}

	// Should we also handle grpc error status codes.
	if grpchook.HookedErrorWrap != nil {
		return grpchook.HookedErrorWrap(grpchook.StatusCode(code), err)
	}

	ce := &codeErr{
		code: code,
	}

	if ee, ok := err.(errsError); ok {
		ce.errsError = ee
	} else {
		ce.errsError = errs.Wrap(err).(errsError)
	}

	return ce
}

// Error wraps the message with a status code into an error.
func Error(code StatusCode, msg string) error {
	return Wrap(code, errs.New("%s", msg))
}

// Errorf : Error :: fmt.Sprintf : fmt.Sprint
func Errorf(code StatusCode, format string, a ...interface{}) error {
	return Wrap(code, errs.New(format, a...))
}

type errsError interface {
	error
	fmt.Formatter
	Name() (string, bool)
}

// codeErr implements error that can work both in grpc and drpc.
type codeErr struct {
	errsError
	code StatusCode
}

func (c *codeErr) Unwrap() error { return c.errsError }
func (c *codeErr) Cause() error  { return c.errsError }

func (c *codeErr) Code() uint64 { return uint64(c.code) }
