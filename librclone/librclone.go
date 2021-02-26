// Package librclone exports shims for C library use
//
// This directory contains code to build rclone as a C library and the
// shims for accessing rclone from C.
//
// The shims are a thin wrapper over the rclone RPC.
//
// Build a shared library like this:
//
//     go build --buildmode=c-shared -o librclone.so github.com/rclone/rclone/librclone
//
// Build a static library like this:
//
//     go build --buildmode=c-archive -o librclone.a github.com/rclone/rclone/librclone
//
// Both the above commands will also generate `librclone.h` which should
// be `#include`d in `C` programs wishing to use the library.
//
// The library will depend on `libdl` and `libpthread`.
package main

import (
	"C"

	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"

	_ "github.com/rclone/rclone/backend/all" // import all backends
	_ "github.com/rclone/rclone/lib/plugin"  // import plugins
)

// RcloneInitialize initializes rclone as a library
//
//export RcloneInitialize
func RcloneInitialize() {
	// TODO: what need to be initialized manually?
}

// RcloneFinalize finalizes the library
//
//export RcloneFinalize
func RcloneFinalize() {
	// TODO: how to clean up? what happens when rcserver terminates?
	// what about unfinished async jobs?
	runtime.GC()
}

// RcloneRPC does a single RPC call. The inputs are (method, input)
// and the output is (output, status). This is an exported interface
// to the rclone API as described in https://rclone.org/rc/
//
//   method is a string, eg "operations/list"
//   input should be a serialized JSON object
//   output will be returned as a serialized JSON object
//   status is a HTTP status return (200=OK anything else fail)
//
// Caller is responsible for freeing the memory for output
//
// Note that when calling from C output and status are returned in an
// RcloneRPC_return which has two members r0 which is output and r1
// which is status.
//
//export RcloneRPC
func RcloneRPC(method *C.char, input *C.char) (output *C.char, status C.int) { //nolint:golint
	res, s := callFunctionJSON(C.GoString(method), C.GoString(input))
	return C.CString(res), C.int(s)
}

// writeError returns a formatted error string and the status passed in
func writeError(path string, in rc.Params, err error, status int) (string, int) {
	fs.Errorf(nil, "rc: %q: error: %v", path, err)
	var w strings.Builder
	// FIXME should factor this
	// Adjust the error return for some well known errors
	errOrig := errors.Cause(err)
	switch {
	case errOrig == fs.ErrorDirNotFound || errOrig == fs.ErrorObjectNotFound:
		status = http.StatusNotFound
	case rc.IsErrParamInvalid(err) || rc.IsErrParamNotFound(err):
		status = http.StatusBadRequest
	}
	// w.WriteHeader(status)
	err = rc.WriteJSON(&w, rc.Params{
		"status": status,
		"error":  err.Error(),
		"input":  in,
		"path":   path,
	})
	if err != nil {
		// can't return the error at this point
		return fmt.Sprintf(`{"error": "rc: failed to write JSON output: %v"}`, err), status
	}
	return w.String(), status
}

// operations/uploadfile and core/command are not supported as they need request or response object
// modified from handlePost in rcserver.go
// call a rc function using JSON to input parameters and output the resulted JSON
func callFunctionJSON(method string, input string) (output string, status int) {
	// create a buffer to capture the output
	in := make(rc.Params)
	err := json.NewDecoder(strings.NewReader(input)).Decode(&in)
	if err != nil {
		// TODO: handle error
		return writeError(method, in, errors.Wrap(err, "failed to read input JSON"), http.StatusBadRequest)
	}

	// Find the call
	call := rc.Calls.Get(method)
	if call == nil {
		return writeError(method, in, errors.Errorf("couldn't find method %q", method), http.StatusNotFound)
	}

	// TODO: handle these cases
	if call.NeedsRequest {
		return writeError(method, in, errors.Errorf("method %q needs request, not supported", method), http.StatusNotFound)
		// Add the request to RC
		//in["_request"] = r
	}
	if call.NeedsResponse {
		return writeError(method, in, errors.Errorf("method %q need response, not supported", method), http.StatusNotFound)
		//in["_response"] = w
	}

	fs.Debugf(nil, "rc: %q: with parameters %+v", method, in)
	// TODO: what is r.Context()? use Background() for the moment
	_, out, err := jobs.NewJob(context.Background(), call.Fn, in)
	if err != nil {
		// handle error
		return writeError(method, in, err, http.StatusInternalServerError)
	}
	if out == nil {
		out = make(rc.Params)
	}

	fs.Debugf(nil, "rc: %q: reply %+v: %v", method, out, err)

	var w strings.Builder
	err = rc.WriteJSON(&w, out)
	if err != nil {
		fs.Errorf(nil, "rc: failed to write JSON output: %v", err)
		return writeError(method, in, err, http.StatusInternalServerError)
	}
	return w.String(), http.StatusOK
}

// do nothing here - necessary for building into a C library
func main() {}
