// Package move provides the move command.
package move

import (
	"context"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/operations/operationsflags"
	"github.com/rclone/rclone/fs/sync"
	"github.com/spf13/cobra"
)

// Globals
var (
	deleteEmptySrcDirs = false
	createEmptySrcDirs = false
	loggerOpt          = operations.LoggerOpt{}
	loggerFlagsOpt     = operationsflags.AddLoggerFlagsOptions{}
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &deleteEmptySrcDirs, "delete-empty-src-dirs", "", deleteEmptySrcDirs, "Delete empty source dirs after move", "")
	flags.BoolVarP(cmdFlags, &createEmptySrcDirs, "create-empty-src-dirs", "", createEmptySrcDirs, "Create empty source dirs on destination after move", "")
	operationsflags.AddLoggerFlags(cmdFlags, &loggerOpt, &loggerFlagsOpt)
	loggerOpt.LoggerFn = operations.NewDefaultLoggerFn(&loggerOpt)
}

var commandDefinition = &cobra.Command{
	Use:   "move source:path dest:path",
	Short: `Move files from source to dest.`,
	// Warning! "|" will be replaced by backticks below
	Long: strings.ReplaceAll(`Moves the contents of the source directory to the destination
directory. Rclone will error if the source and destination overlap and
the remote does not support a server-side directory move operation.

To move single files, use the [moveto](/commands/rclone_moveto/)
command instead.

If no filters are in use and if possible this will server-side move
|source:path| into |dest:path|. After this |source:path| will no
longer exist.

Otherwise for each file in |source:path| selected by the filters (if
any) this will move it into |dest:path|.  If possible a server-side
move will be used, otherwise it will copy it (server-side if possible)
into |dest:path| then delete the original (if no errors on copy) in
|source:path|.

If you want to delete empty source directories after move, use the
|--delete-empty-src-dirs| flag.

See the [--no-traverse](/docs/#no-traverse) option for controlling
whether rclone lists the destination directory or not.  Supplying this
option when moving a small number of files into a large destination
can speed transfers up greatly.

Rclone will sync the modification times of files and directories if
the backend supports it. If metadata syncing is required then use the
|--metadata| flag.

Note that the modification time and metadata for the root directory
will **not** be synced. See https://github.com/rclone/rclone/issues/7652
for more info.

**Important**: Since this can cause data loss, test first with the
|--dry-run| or the |--interactive|/|-i| flag.

**Note**: Use the |-P|/|--progress| flag to view real-time transfer statistics.

## Logger Flags

The ` + "`--differ`" + `, ` + "`--missing-on-dst`" + `, ` + "`--missing-on-src`" + `, ` +
		"`--match`" + ` and ` + "`--error`" + ` flags write paths, one per line, to the file name (or
stdout if it is ` + "`-`" + `) supplied. What they write is described in the
help below. For example ` + "`--differ`" + ` will write all paths which are
present on both the source and destination but different.

The ` + "`--combined`" + ` flag will write a file (or stdout) which contains all
file paths with a symbol and then a space and then the path to tell
you what happened to it. These are reminiscent of diff files.

- ` + "`= path`" + ` means path was found in source and destination and was identical
- ` + "`- path`" + ` means path was missing on the source, so only in the destination
- ` + "`+ path`" + ` means path was missing on the destination, so only in the source
- ` + "`* path`" + ` means path was present in source and destination but different.
- ` + "`! path`" + ` means there was an error reading or hashing the source or dest.

The ` + "`--dest-after`" + ` flag writes a list file using the same format flags
as [` + "`lsf`" + `](/commands/rclone_lsf/#synopsis) (including [customizable options
for hash, modtime, etc.](/commands/rclone_lsf/#synopsis))
Conceptually it is similar to rsync's ` + "`--itemize-changes`" + `, but not identical
-- it should output an accurate list of what will be on the destination
after the move.

When the ` + "`--no-traverse`" + ` flag is set, all logs involving files that exist only
on the destination will be incomplete or completely missing.

Note that these logger flags have a few limitations, and certain scenarios
are not currently supported:

- ` + "`--max-duration`" + ` / ` + "`CutoffModeHard`" + `
- ` + "`--compare-dest`" + ` / ` + "`--copy-dest`" + `
- server-side moves of an entire dir at once
- High-level retries, because there would be duplicates (use ` + "`--retries 1`" + ` to disable)
- Possibly some unusual error scenarios

Note also that each file is logged during the move, as opposed to after, so it
is most useful as a predictor of what SHOULD happen to each file
(which may or may not match what actually DID.)
`, "|", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.19",
		"groups":            "Filter,Listing,Important,Copy",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
		cmd.Run(true, true, command, func() error {
			ctx := context.Background()
			close, err := operationsflags.ConfigureLoggers(ctx, fdst, command, &loggerOpt, loggerFlagsOpt)
			if err != nil {
				return err
			}
			defer close()

			if loggerFlagsOpt.AnySet() {
				ctx = operations.WithSyncLogger(ctx, loggerOpt)
			}

			if srcFileName == "" {
				return sync.MoveDir(ctx, fdst, fsrc, deleteEmptySrcDirs, createEmptySrcDirs)
			}
			return operations.MoveFile(ctx, fdst, fsrc, srcFileName, srcFileName)
		})
	},
}
