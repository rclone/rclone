// Rclone as a wasm library
//
// This library exports the core rc functionality

//go:build js

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"syscall/js"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"

	// Core functionality we need
	_ "github.com/rclone/rclone/fs/operations"
	_ "github.com/rclone/rclone/fs/sync"

	//	_ "github.com/rclone/rclone/backend/all" // import all backends

	// Backends
	_ "github.com/rclone/rclone/backend/memory"
)

var (
	document js.Value
	jsJSON   js.Value
)

func getElementById(name string) js.Value {
	node := document.Call("getElementById", name)
	if node.IsUndefined() {
		log.Fatalf("Couldn't find element %q", name)
	}
	return node
}

func time() int {
	return js.Global().Get("Date").New().Call("getTime").Int()
}

func paramToValue(in rc.Params) (out js.Value) {
	return js.Value{}
}

// errorValue turns an error into a js.Value
func errorValue(method string, in js.Value, err error) js.Value {
	fs.Errorf(nil, "rc: %q: error: %v", method, err)
	// Adjust the error return for some well known errors
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, fs.ErrorDirNotFound) || errors.Is(err, fs.ErrorObjectNotFound):
		status = http.StatusNotFound
	case rc.IsErrParamInvalid(err) || rc.IsErrParamNotFound(err):
		status = http.StatusBadRequest
	}
	return js.ValueOf(map[string]interface{}{
		"status": status,
		"error":  err.Error(),
		"input":  in,
		"path":   method,
	})
}

// rcCallback is a callback for javascript to access the api
//
// FIXME should this should return a promise so we can return errors properly?
func rcCallback(this js.Value, args []js.Value) interface{} {
	ctx := context.Background() // FIXME
	log.Printf("rcCallback: this=%v args=%v", this, args)

	if len(args) != 2 {
		return errorValue("", js.Undefined(), errors.New("need two parameters to rc call"))
	}
	method := args[0].String()
	inRaw := args[1]
	var in = rc.Params{}
	switch inRaw.Type() {
	case js.TypeNull:
	case js.TypeObject:
		inJSON := jsJSON.Call("stringify", inRaw).String()
		err := json.Unmarshal([]byte(inJSON), &in)
		if err != nil {
			return errorValue(method, inRaw, fmt.Errorf("couldn't unmarshal input: %w", err))
		}
	default:
		return errorValue(method, inRaw, errors.New("in parameter must be null or object"))
	}

	call := rc.Calls.Get(method)
	if call == nil {
		return errorValue(method, inRaw, fmt.Errorf("method %q not found", method))
	}

	out, err := call.Fn(ctx, in)
	if err != nil {
		return errorValue(method, inRaw, fmt.Errorf("method call failed: %w", err))
	}
	if out == nil {
		return nil
	}
	var out2 map[string]interface{}
	err = rc.Reshape(&out2, out)
	if err != nil {
		return errorValue(method, inRaw, fmt.Errorf("result reshape failed: %w", err))
	}

	return js.ValueOf(out2)
}

func main() {
	log.Printf("Running on goos/goarch = %s/%s", runtime.GOOS, runtime.GOARCH)
	if js.Global().IsUndefined() {
		log.Fatalf("Didn't find Global - not running in browser")
	}
	document = js.Global().Get("document")
	if document.IsUndefined() {
		log.Fatalf("Didn't find document - not running in browser")
	}

	jsJSON = js.Global().Get("JSON")
	if jsJSON.IsUndefined() {
		log.Fatalf("can't find JSON")
	}

	// Set rc
	js.Global().Set("rc", js.FuncOf(rcCallback))

	// Signal that it is valid
	rcValidResolve := js.Global().Get("rcValidResolve")
	if rcValidResolve.IsUndefined() {
		log.Fatalf("Didn't find rcValidResolve")
	}
	rcValidResolve.Invoke()

	// Wait forever
	select {}
}
