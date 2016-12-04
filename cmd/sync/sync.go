package sync

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "sync source:path dest:path",
	Short: `Make source and dest identical, modifying destination only.`,
	Long: `
Sync the source to the destination, changing the destination
only.  Doesn't transfer unchanged files, testing by size and
modification time or MD5SUM.  Destination is updated to match
source, including deleting files if necessary.

**Important**: Since this can cause data loss, test first with the
` + "`" + `--dry-run` + "`" + ` flag to see exactly what would be copied and deleted.

Note that files in the destination won't be deleted if there were any
errors at any point.

It is always the contents of the directory that is synced, not the
directory so when source:path is a directory, it's the contents of
source:path that are copied, not the directory name and contents.  See
extended explanation in the ` + "`" + `copy` + "`" + ` command above if unsure.

If dest:path doesn't exist, it is created and the source:path contents
go there.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, fdst := cmd.NewFsSrcDst(args)
		cmd.Run(true, true, command, func() error {
			return fs.Sync(fdst, fsrc)
		})
	},
}
