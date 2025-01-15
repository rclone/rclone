// Package move provides the move command.
package move

import (
	"context"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/spf13/cobra"
)

// Globals
var (
	deleteEmptySrcDirs = false
	createEmptySrcDirs = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &deleteEmptySrcDirs, "delete-empty-src-dirs", "", deleteEmptySrcDirs, "Delete empty source dirs after move", "")
	flags.BoolVarP(cmdFlags, &createEmptySrcDirs, "create-empty-src-dirs", "", createEmptySrcDirs, "Create empty source dirs on destination after move", "")
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
`, "|", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.19",
		"groups":            "Filter,Listing,Important,Copy",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
		cmd.Run(true, true, command, func() error {
			if srcFileName == "" {
				return sync.MoveDir(context.Background(), fdst, fsrc, deleteEmptySrcDirs, createEmptySrcDirs)
			}
			return operations.MoveFile(context.Background(), fdst, fsrc, srcFileName, srcFileName)
		})
	},
}
