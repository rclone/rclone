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
	"github.com/rclone/rclone/librclone/librclone"

	_ "github.com/rclone/rclone/backend/all" // import all backends
	_ "github.com/rclone/rclone/lib/plugin"  // import plugins
)

// RcloneInitialize initializes rclone as a library
//
//export RcloneInitialize
func RcloneInitialize() {
	librclone.Initialize()
}

// RcloneFinalize finalizes the library
//
//export RcloneFinalize
func RcloneFinalize() {
	librclone.Finalize()
}

// RcloneRPCResult is returned from RcloneRPC
//
//   Output will be returned as a serialized JSON object
//   Status is a HTTP status return (200=OK anything else fail)
type RcloneRPCResult struct { //nolint:deadcode
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
	output, status := librclone.RPC(C.GoString(method), C.GoString(input))
	result.Output = C.CString(output)
	result.Status = C.int(status)
	return result
}

// do nothing here - necessary for building into a C library
func main() {}
