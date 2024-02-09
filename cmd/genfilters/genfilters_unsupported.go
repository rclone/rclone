// Build for genfilters for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || js
// +build plan9 js

package genfilters

import (
	"context"
	"fmt"
	"runtime"

	"github.com/rclone/rclone/fs"
)

// this is just here to prevent build errors on unsupported platforms

// GenFilters is the main entry point. It shows a navigable tree view of the current directory and generates filters.
func GenFilters(ctx context.Context, f fs.Fs, infile, outfile string) error {
	return fmt.Errorf("genfilters command is not supported on your platform (%v)", runtime.GOOS)
}
