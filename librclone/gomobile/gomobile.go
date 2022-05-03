// Package gomobile exports shims for gomobile use
package gomobile

import (
	"github.com/rclone/rclone/librclone/librclone"

	_ "github.com/rclone/rclone/backend/all" // import all backends
	_ "github.com/rclone/rclone/lib/plugin"  // import plugins

	_ "golang.org/x/mobile/event/key" // make go.mod add this as a dependency
)

// RcloneInitialize initializes rclone as a library
func RcloneInitialize() {
	librclone.Initialize()
}

// RcloneFinalize finalizes the library
func RcloneFinalize() {
	librclone.Finalize()
}

// RcloneRPCResult is returned from RcloneRPC
//
//   Output will be returned as a serialized JSON object
//   Status is a HTTP status return (200=OK anything else fail)
type RcloneRPCResult struct {
	Output string
	Status int
}

// RcloneRPC has an interface optimised for gomobile, in particular
// the function signature is valid under gobind rules.
//
// https://pkg.go.dev/golang.org/x/mobile/cmd/gobind#hdr-Type_restrictions
func RcloneRPC(method string, input string) (result *RcloneRPCResult) { //nolint:deadcode
	output, status := librclone.RPC(method, input)
	return &RcloneRPCResult{
		Output: output,
		Status: status,
	}
}
