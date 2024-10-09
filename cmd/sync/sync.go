// Package sync provides the sync command.
package sync

import (
	"context"
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
	flags.BoolVarP(cmdFlags, &createEmptySrcDirs, "create-empty-src-dirs", "", createEmptySrcDirs, "Create empty source dirs on destination after sync", "")
	operationsflags.AddLoggerFlags(cmdFlags, &loggerOpt, &loggerFlagsOpt)
	loggerOpt.LoggerFn = operations.NewDefaultLoggerFn(&loggerOpt)
}

var commandDefinition = &cobra.Command{
	Use:   "sync source:path dest:path",
	Short: `Make source and dest identical, modifying destination only.`,
	Long: `Sync the source to the destination, changing the destination
only.  Doesn't transfer files that are identical on source and
destination, testing by size and modification time or MD5SUM.
Destination is updated to match source, including deleting files
if necessary (except duplicate objects, see below). If you don't
want to delete files from destination, use the
[copy](/commands/rclone_copy/) command instead.

**Important**: Since this can cause data loss, test first with the
` + "`--dry-run` or the `--interactive`/`-i`" + ` flag.

    rclone sync --interactive SOURCE remote:DESTINATION

Note that files in the destination won't be deleted if there were any
errors at any point.  Duplicate objects (files with the same name, on
those providers that support it) are also not yet handled.

It is always the contents of the directory that is synced, not the
directory itself. So when source:path is a directory, it's the contents of
source:path that are copied, not the directory name and contents.  See
extended explanation in the [copy](/commands/rclone_copy/) command if unsure.

If dest:path doesn't exist, it is created and the source:path contents
go there.

It is not possible to sync overlapping remotes. However, you may exclude
the destination from the sync with a filter rule or by putting an 
exclude-if-present file inside the destination directory and sync to a
destination that is inside the source directory.

Rclone will sync the modification times of files and directories if
the backend supports it. If metadata syncing is required then use the
` + "`--metadata`" + ` flag.

Note that the modification time and metadata for the root directory
will **not** be synced. See https://github.com/rclone/rclone/issues/7652
for more info.

**Note**: Use the ` + "`-P`" + `/` + "`--progress`" + ` flag to view real-time transfer statistics

**Note**: Use the ` + "`rclone dedupe`" + ` command to deal with "Duplicate object/directory found in source/destination - ignoring" errors.
See [this forum post](https://forum.rclone.org/t/sync-not-clearing-duplicates/14372) for more info.

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
after the sync.

Note that these logger flags have a few limitations, and certain scenarios
are not currently supported:

- ` + "`--max-duration`" + ` / ` + "`CutoffModeHard`" + `
- ` + "`--compare-dest`" + ` / ` + "`--copy-dest`" + `
- server-side moves of an entire dir at once
- High-level retries, because there would be duplicates (use ` + "`--retries 1`" + ` to disable)
- Possibly some unusual error scenarios

Note also that each file is logged during the sync, as opposed to after, so it
is most useful as a predictor of what SHOULD happen to each file
(which may or may not match what actually DID.)
`,
	Annotations: map[string]string{
		"groups": "Sync,Copy,Filter,Listing,Important",
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
				return sync.Sync(ctx, fdst, fsrc, createEmptySrcDirs)
			}
			return operations.CopyFile(ctx, fdst, fsrc, srcFileName, srcFileName)
		})
	},
}
