// Build for genfilters for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || js || aix

package genfilters

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/rclone/rclone/fs"
)

// Options configures Run
type Options struct {
	Edit          string // filter file to edit; if empty read stdin and write stdout
	ExcludeOthers bool   // start excluding everything without a rule
	Open          bool   // open the edited file when done
	Regex         bool   // also write the rules as regular expressions to stderr
	Cmd           bool   // also write the rules as --filter flags to stderr
}

// ErrCancelled is returned by Run when the user quits without writing the rules
var ErrCancelled = errors.New("cancelled: the filter rules were not written")

// Run shows the user interface for making filter rules for f
func Run(ctx context.Context, f fs.Fs, opt Options) error {
	return fmt.Errorf("genfilters is not supported on %s", runtime.GOOS)
}
