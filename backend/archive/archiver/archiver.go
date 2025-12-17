// Package archiver registers all the archivers
package archiver

import (
	"context"

	"github.com/rclone/rclone/fs"
)

// Archiver describes an archive package
type Archiver struct {
	// New constructs an Fs from the (wrappedFs, remote) with the objects
	// prefix with prefix and rooted at root
	New       func(ctx context.Context, f fs.Fs, remote, prefix, root string) (fs.Fs, error)
	Extension string
}

// Archivers is a slice of all registered archivers
var Archivers []Archiver

// Register adds the archivers provided to the list of known archivers
func Register(as ...Archiver) {
	Archivers = append(Archivers, as...)
}
