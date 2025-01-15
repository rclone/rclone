// Package cat provides the cat command.
package cat

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

// Globals
var (
	head      = int64(0)
	tail      = int64(0)
	offset    = int64(0)
	count     = int64(-1)
	discard   = false
	separator = string("")
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.Int64VarP(cmdFlags, &head, "head", "", head, "Only print the first N characters", "")
	flags.Int64VarP(cmdFlags, &tail, "tail", "", tail, "Only print the last N characters", "")
	flags.Int64VarP(cmdFlags, &offset, "offset", "", offset, "Start printing at offset N (or from end if -ve)", "")
	flags.Int64VarP(cmdFlags, &count, "count", "", count, "Only print N characters", "")
	flags.BoolVarP(cmdFlags, &discard, "discard", "", discard, "Discard the output instead of printing", "")
	flags.StringVarP(cmdFlags, &separator, "separator", "", separator, "Separator to use between objects when printing multiple files", "")
}

var commandDefinition = &cobra.Command{
	Use:   "cat remote:path",
	Short: `Concatenates any files and sends them to stdout.`,
	// Warning! "|" will be replaced by backticks below
	Long: strings.ReplaceAll(`Sends any files to standard output.

You can use it like this to output a single file

    rclone cat remote:path/to/file

Or like this to output any file in dir or its subdirectories.

    rclone cat remote:path/to/dir

Or like this to output any .txt files in dir or its subdirectories.

    rclone --include "*.txt" cat remote:path/to/dir

Use the |--head| flag to print characters only at the start, |--tail| for
the end and |--offset| and |--count| to print a section in the middle.
Note that if offset is negative it will count from the end, so
|--offset -1 --count 1| is equivalent to |--tail 1|.

Use the |--separator| flag to print a separator value between files. Be sure to
shell-escape special characters. For example, to print a newline between
files, use:

* bash:

      rclone --include "*.txt" --separator $'\n' cat remote:path/to/dir

* powershell:

      rclone --include "*.txt" --separator "|n" cat remote:path/to/dir
`, "|", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.33",
		"groups":            "Filter,Listing",
	},
	Run: func(command *cobra.Command, args []string) {
		usedOffset := offset != 0 || count >= 0
		usedHead := head > 0
		usedTail := tail > 0
		if usedHead && usedTail || usedHead && usedOffset || usedTail && usedOffset {
			fs.Fatalf(nil, "Can only use one of  --head, --tail or --offset with --count")
		}
		if head > 0 {
			offset = 0
			count = head
		}
		if tail > 0 {
			offset = -tail
			count = -1
		}
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		var w io.Writer = os.Stdout
		if discard {
			w = io.Discard
		}
		cmd.Run(false, false, command, func() error {
			return operations.Cat(context.Background(), fsrc, w, offset, count, []byte(separator))
		})
	},
}
