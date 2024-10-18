// Package copy provides the copy command.
package copy

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

var (
	createEmptySrcDirs = false
	loggerOpt          = operations.LoggerOpt{}
	loggerFlagsOpt     = operationsflags.AddLoggerFlagsOptions{}
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &createEmptySrcDirs, "create-empty-src-dirs", "", createEmptySrcDirs, "Create empty source dirs on destination after copy", "")
	operationsflags.AddLoggerFlags(cmdFlags, &loggerOpt, &loggerFlagsOpt)
	loggerOpt.LoggerFn = operations.NewDefaultLoggerFn(&loggerOpt)
}

var commandDefinition = &cobra.Command{
	Use:   "copy source:path dest:path",
	Short: `Copy files from source to dest, skipping identical files.`,
	// Note: "|" will be replaced by backticks below
	Long: strings.ReplaceAll(`Copy the source to the destination.  Does not transfer files that are
identical on source and destination, testing by size and modification
time or MD5SUM.  Doesn't delete files from the destination. If you
want to also delete files from destination, to make it match source,
use the [sync](/commands/rclone_sync/) command instead.

Note that it is always the contents of the directory that is synced,
not the directory itself. So when source:path is a directory, it's the
contents of source:path that are copied, not the directory name and
contents.

To copy single files, use the [copyto](/commands/rclone_copyto/)
command instead.

If dest:path doesn't exist, it is created and the source:path contents
go there.

For example

    rclone copy source:sourcepath dest:destpath

Let's say there are two files in sourcepath

    sourcepath/one.txt
    sourcepath/two.txt

This copies them to

    destpath/one.txt
    destpath/two.txt

Not to

    destpath/sourcepath/one.txt
    destpath/sourcepath/two.txt

If you are familiar with |rsync|, rclone always works as if you had
written a trailing |/| - meaning "copy the contents of this directory".
This applies to all commands and whether you are talking about the
source or destination.

See the [--no-traverse](/docs/#no-traverse) option for controlling
whether rclone lists the destination directory or not.  Supplying this
option when copying a small number of files into a large destination
can speed transfers up greatly.

For example, if you have many files in /path/to/src but only a few of
them change every day, you can copy all the files which have changed
recently very efficiently like this:

    rclone copy --max-age 24h --no-traverse /path/to/src remote:


Rclone will sync the modification times of files and directories if
the backend supports it. If metadata syncing is required then use the
|--metadata| flag.

Note that the modification time and metadata for the root directory
will **not** be synced. See https://github.com/rclone/rclone/issues/7652
for more info.

**Note**: Use the |-P|/|--progress| flag to view real-time transfer statistics.

**Note**: Use the |--dry-run| or the |--interactive|/|-i| flag to test without copying anything.

## Logger Flags

The |--differ|, |--missing-on-dst|, |--missing-on-src|, |--match| and |--error| flags write paths,
one per line, to the file name (or stdout if it is |-|) supplied. What they write is described
in the help below. For example |--differ| will write all paths which are present
on both the source and destination but different.

The |--combined| flag will write a file (or stdout) which contains all
file paths with a symbol and then a space and then the path to tell
you what happened to it. These are reminiscent of diff files.

- |= path| means path was found in source and destination and was identical
- |- path| means path was missing on the source, so only in the destination
- |+ path| means path was missing on the destination, so only in the source
- |* path| means path was present in source and destination but different.
- |! path| means there was an error reading or hashing the source or dest.

The |--dest-after| flag writes a list file using the same format flags
as [|lsf|](/commands/rclone_lsf/#synopsis) (including [customizable options
for hash, modtime, etc.](/commands/rclone_lsf/#synopsis))
Conceptually it is similar to rsync's |--itemize-changes|, but not identical
-- it should output an accurate list of what will be on the destination
after the copy.

When the |--no-traverse| flag is set, all logs involving files that exist only
on the destination will be incomplete or completely missing.

Note that these logger flags have a few limitations, and certain scenarios
are not currently supported:

- |--max-duration| / |CutoffModeHard|
- |--compare-dest| / |--copy-dest|
- server-side moves of an entire dir at once
- High-level retries, because there would be duplicates (use |--retries 1| to disable)
- Possibly some unusual error scenarios

Note also that each file is logged during the copy, as opposed to after, so it
is most useful as a predictor of what SHOULD happen to each file
(which may or may not match what actually DID.)
`, "|", "`"),
	Annotations: map[string]string{
		"groups": "Copy,Filter,Listing,Important",
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
				return sync.CopyDir(ctx, fdst, fsrc, createEmptySrcDirs)
			}
			return operations.CopyFile(ctx, fdst, fsrc, srcFileName, srcFileName)
		})
	},
}
