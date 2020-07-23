package sync

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/spf13/cobra"
)

var (
	createEmptySrcDirs = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &createEmptySrcDirs, "create-empty-src-dirs", "", createEmptySrcDirs, "Create empty source dirs on destination after sync")
}

var commandDefinition = &cobra.Command{
	Use:   "sync source:path dest:path",
	Short: `Make source and dest identical, modifying destination only.`,
	Long: `
Sync the source to the destination, changing the destination
only.  Doesn't transfer unchanged files, testing by size and
modification time or MD5SUM.  Destination is updated to match
source, including deleting files if necessary (except duplicate
objects, see below).

**Important**: Since this can cause data loss, test first with the
` + "`--dry-run` or the `--interactive`/`-i`" + ` flag.

    rclone sync -i SOURCE remote:DESTINATION

Note that files in the destination won't be deleted if there were any
errors at any point.  Duplicate objects (files with the same name, on
those providers that support it) are also not yet handled.

It is always the contents of the directory that is synced, not the
directory so when source:path is a directory, it's the contents of
source:path that are copied, not the directory name and contents.  See
extended explanation in the ` + "`" + `copy` + "`" + ` command above if unsure.

If dest:path doesn't exist, it is created and the source:path contents
go there.

**Note**: Use the ` + "`-P`" + `/` + "`--progress`" + ` flag to view real-time transfer statistics

**Note**: Use the ` + "`rclone dedupe`" + ` command to deal with "Duplicate object/directory found in source/destination - ignoring" errors.
See [this forum post](https://forum.rclone.org/t/sync-not-clearing-duplicates/14372) for more info.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
		cmd.Run(true, true, command, func() error {
			if srcFileName == "" {
				return sync.Sync(context.Background(), fdst, fsrc, createEmptySrcDirs)
			}
			return operations.CopyFile(context.Background(), fdst, fsrc, srcFileName, srcFileName)
		})
	},
}
