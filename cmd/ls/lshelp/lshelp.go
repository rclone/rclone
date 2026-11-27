// Package lshelp provides common help for list commands.
package lshelp

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/pflag"
)

// OutputFile is the file to write the listing to, or "" for standard output.
var OutputFile string

// AddOutputFileFlag adds the --output-file flag to a list command.
func AddOutputFileFlag(cmdFlags *pflag.FlagSet) {
	flags.StringVarP(cmdFlags, &OutputFile, "output-file", "", "", "Write the listing to a file rather than standard output", "")
}

// Output returns the writer for the listing: the --output-file if one
// was given, otherwise standard output. Call close when done.
func Output() (out io.Writer, close func(), err error) {
	if OutputFile == "" {
		return os.Stdout, func() {}, nil
	}
	file, err := os.Create(OutputFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create output file %q: %w", OutputFile, err)
	}
	close = func() {
		if err := file.Close(); err != nil {
			fs.Errorf(nil, "Failed to close output file %q: %v", OutputFile, err)
		}
	}
	return file, close, nil
}

// Help describes the common help for all the list commands
// Warning! "|" will be replaced by backticks below
var Help = strings.ReplaceAll(`Any of the filtering options can be applied to this command.

There are several related list commands

- |ls| to list size and path of objects only
- |lsl| to list modification time, size and path of objects only
- |lsd| to list directories only
- |lsf| to list objects and directories in easy to parse format
- |lsjson| to list objects and directories in JSON format

|ls|,|lsl|,|lsd| are designed to be human-readable.
|lsf| is designed to be human and machine-readable.
|lsjson| is designed to be machine-readable.

Note that |ls| and |lsl| recurse by default - use |--max-depth 1| to stop the recursion.

The other list commands |lsd|,|lsf|,|lsjson| do not recurse by default -
use |-R| to make them recurse.

On the bucket-based remotes (e.g. s3, gcs, b2) a directory is a key prefix
rather than an object, so a listing without |-R| shows each prefix as one
directory entry and not the objects stored under it.

List commands prefer a recursive method that uses more memory but fewer
transactions by default. Use |--disable ListR| to suppress the behavior.
See [|--fast-list|](/docs/#fast-list) for more details.

Listing a nonexistent directory will produce an error except for
remotes which can't have empty directories (e.g. s3, swift, or gcs -
the bucket-based remotes).

Use |--output-file| to write the listing to a file instead of standard
output. Progress (|-P|) and log output still go to the terminal, so a
listing can be captured while watching the progress.`, "|", "`")
