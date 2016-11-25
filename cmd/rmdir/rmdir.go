package rmdir

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(rmdirCmd)
}

var rmdirCmd = &cobra.Command{
	Use:   "rmdir remote:path",
	Short: `Remove the path if empty.`,
	Long: `
Remove the path.  Note that you can't remove a path with
objects in it, use purge for that.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDst(args)
		cmd.Run(true, command, func() error {
			return fs.Rmdir(fdst, "")
		})
	},
}
