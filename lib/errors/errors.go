// Package errors provides error handling utilities.
package errors

import (
	"reflect"
)

// WalkFunc is the signature of the Walk callback function. The function gets the
// current error in the chain and should return true if the chain processing
// should be aborted.
type WalkFunc func(error) bool

// Walk invokes the given function for each error in the chain. If the
// provided functions returns true or no further cause can be found, the process
// is stopped and no further calls will be made.
//
// The next error in the chain is determined by the following rules:
//
//		the return value of this method is used.
//	  - If the current error has a `Unwrap() error` method
//	    the return value of this method is used.
//	  - If the current error has a `Unwrap() []error` method
//	    the return values of this method is used.
//	  - Common errors in the Go runtime that contain an Err field will use this value.
func Walk(err error, f WalkFunc) {
	for prev := err; err != nil; prev = err {
		if f(err) {
			return
		}

		switch e := err.(type) {
		case multiWrapper:
			for _, err = range e.Unwrap() {
				Walk(err, f)
			}
			return
		case causer:
			err = e.Cause()
		case wrapper:
			err = e.Unwrap()
		default:
			// Unpack any struct or *struct with a field of name Err which satisfies
			// the error interface. This includes *url.Error, *net.OpError,
			// *os.SyscallError and many others in the stdlib.
			errType := reflect.TypeOf(err)
			errValue := reflect.ValueOf(err)
			if errValue.IsValid() && errType.Kind() == reflect.Ptr {
				errType = errType.Elem()
				errValue = errValue.Elem()
			}
			if errValue.IsValid() && errType.Kind() == reflect.Struct {
				if errField := errValue.FieldByName("Err"); errField.IsValid() {
					errFieldValue := errField.Interface()
					if newErr, ok := errFieldValue.(error); ok {
						err = newErr
					}
				}
			}
		}
		if reflect.DeepEqual(err, prev) {
			break
		}
	}
}

type causer interface {
	Cause() error
}
type wrapper interface {
	Unwrap() error
}
type multiWrapper interface {
	Unwrap() []error
}
