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

/*
struct RcloneRPCResult {
	char*	Output;
	int	Status;
};
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"

	_ "github.com/rclone/rclone/backend/all" // import all backends
	_ "github.com/rclone/rclone/lib/plugin"  // import plugins
)

// RcloneInitialize initializes rclone as a library
//
//export RcloneInitialize
func RcloneInitialize() {
	// A subset of initialisation copied from cmd.go
	// Note that we don't want to pull in anything which depends on pflags

	ctx := context.Background()

	// Start the logger
	log.InitLogging()

	// Load the config - this may need to be configurable
	configfile.Install()

	// Start accounting
	accounting.Start(ctx)
}

// RcloneFinalize finalizes the library
//
//export RcloneFinalize
func RcloneFinalize() {
	// TODO: how to clean up? what happens when rcserver terminates?
	// what about unfinished async jobs?
	runtime.GC()
}

// RcloneRPCResult is returned from RcloneRPC
//
//   Output will be returned as a serialized JSON object
//   Status is a HTTP status return (200=OK anything else fail)
type RcloneRPCResult struct {
	Output *C.char
	Status C.int
}

// RcloneRPC does a single RPC call. The inputs are (method, input)
// and the output is (output, status). This is an exported interface
// to the rclone API as described in https://rclone.org/rc/
//
//   method is a string, eg "operations/list"
//   input should be a serialized JSON object
//   result.Output will be returned as a serialized JSON object
//   result.Status is a HTTP status return (200=OK anything else fail)
//
// Caller is responsible for freeing the memory for result.Output,
// result itself is passed on the stack.
//
//export RcloneRPC
func RcloneRPC(method *C.char, input *C.char) (result C.struct_RcloneRPCResult) { //nolint:golint
	output, status := callFunctionJSON(C.GoString(method), C.GoString(input))
	result.Output = C.CString(output)
	result.Status = C.int(status)
	return result
}

// RcloneMobileRPCResult is returned from RcloneMobileRPC
//
//   Output will be returned as a serialized JSON object
//   Status is a HTTP status return (200=OK anything else fail)
type RcloneMobileRPCResult struct {
	Output string
	Status int
}

// RcloneMobileRPCRPC this works the same as RcloneRPC but has an interface
// optimised for gomobile, in particular the function signature is
// valid under gobind rules.
//
// https://pkg.go.dev/golang.org/x/mobile/cmd/gobind#hdr-Type_restrictions
func RcloneMobileRPCRPC(method string, input string) (result RcloneMobileRPCResult) {
	output, status := callFunctionJSON(method, input)
	result.Output = output
	result.Status = status
	return result
}

// writeError returns a formatted error string and the status passed in
func writeError(path string, in rc.Params, err error, status int) (string, int) {
	fs.Errorf(nil, "rc: %q: error: %v", path, err)
	params, status := rc.Error(path, in, err, status)
	var w strings.Builder
	err = rc.WriteJSON(&w, params)
	if err != nil {
		// ultimate fallback error
		fs.Errorf(nil, "writeError: failed to write JSON output from %#v: %v", in, err)
		status = http.StatusInternalServerError
		w.Reset()
		fmt.Fprintf(&w, `{
	"error": %q,
	"path": %q,
	"status": %d
}`, err, path, status)

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

	_, out, err := jobs.NewJob(context.Background(), call.Fn, in)
	if err != nil {
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
