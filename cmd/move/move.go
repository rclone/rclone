package move

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "move source:path dest:path",
	Short: `Move files from source to dest.`,
	Long: `
Moves the contents of the source directory to the destination
directory. Rclone will error if the source and destination overlap and
the remote does not support a server side directory move operation.

If no filters are in use and if possible this will server side move
` + "`source:path`" + ` into ` + "`dest:path`" + `. After this ` + "`source:path`" + ` will no
longer longer exist.

Otherwise for each file in ` + "`source:path`" + ` selected by the filters (if
any) this will move it into ` + "`dest:path`" + `.  If possible a server side
move will be used, otherwise it will copy it (server side if possible)
into ` + "`dest:path`" + ` then delete the original (if no errors on copy) in
` + "`source:path`" + `.

**Important**: Since this can cause data loss, test first with the
--dry-run flag.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, fdst := cmd.NewFsSrcDst(args)
		cmd.Run(true, true, command, func() error {
			return fs.MoveDir(fdst, fsrc)
		})
	},
}
