// Package librclone exports shims for library use
//
// This is the internal implementation which is used for C and
// Gomobile libraries which need slightly different export styles.
//
// The shims are a thin wrapper over the rclone RPC.
package librclone

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
)

// Initialize initializes rclone as a library
//
//export Initialize
func Initialize() {
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

// Finalize finalizes the library
func Finalize() {
	// TODO: how to clean up? what happens when rcserver terminates?
	// what about unfinished async jobs?
	runtime.GC()
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

// RPC runs a transaction over the RC
//
// Calling an rc function using JSON to input parameters and output the resulted JSON.
//
// operations/uploadfile and core/command are not supported as they need request or response object
// modified from handlePost in rcserver.go
func RPC(method string, input string) (output string, status int) {
	in := make(rc.Params)

	// Catch panics
	defer func() {
		if r := recover(); r != nil {
			output, status = writeError(method, in, fmt.Errorf("panic: %v\n%s", r, debug.Stack()), http.StatusInternalServerError)
			return
		}
	}()

	// create a buffer to capture the output
	if input != "" {
		err := json.NewDecoder(strings.NewReader(input)).Decode(&in)
		if err != nil {
			return writeError(method, in, fmt.Errorf("failed to read input JSON: %w", err), http.StatusBadRequest)
		}
	}

	// Find the call
	call := rc.Calls.Get(method)
	if call == nil {
		return writeError(method, in, fmt.Errorf("couldn't find method %q", method), http.StatusNotFound)
	}

	// TODO: handle these cases
	if call.NeedsRequest {
		return writeError(method, in, fmt.Errorf("method %q needs request, not supported", method), http.StatusNotFound)
		// Add the request to RC
		//in["_request"] = r
	}
	if call.NeedsResponse {
		return writeError(method, in, fmt.Errorf("method %q need response, not supported", method), http.StatusNotFound)
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
