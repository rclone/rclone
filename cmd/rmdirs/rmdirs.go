package rmdir

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	leaveRoot = false
)

func init() {
	cmd.Root.AddCommand(rmdirsCmd)
	rmdirsCmd.Flags().BoolVarP(&leaveRoot, "leave-root", "", leaveRoot, "Do not remove root directory if empty")
}

var rmdirsCmd = &cobra.Command{
	Use:   "rmdirs remote:path",
	Short: `Remove empty directories under the path.`,
	Long: `This removes any empty directories (or directories that only contain
empty directories) under the path that it finds, including the path if
it has nothing in.

If you supply the --leave-root flag, it will not remove the root directory.

This is useful for tidying up remotes that rclone has left a lot of
empty directories in.

`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDir(args)
		cmd.Run(true, false, command, func() error {
			return operations.Rmdirs(context.Background(), fdst, "", leaveRoot)
		})
	},
}
