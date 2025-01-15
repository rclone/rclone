// Package rmdir provides the rmdir command.
package rmdir

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "rmdir remote:path",
	Short: `Remove the empty directory at path.`,
	Long: `This removes empty directory given by path. Will not remove the path if it
has any objects in it, not even empty subdirectories. Use
command [rmdirs](/commands/rclone_rmdirs/) (or [delete](/commands/rclone_delete/)
with option ` + "`--rmdirs`" + `) to do that.

To delete a path and any objects in it, use [purge](/commands/rclone_purge/) command.
`,
	Annotations: map[string]string{
		"groups": "Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDir(args)
		cmd.Run(true, false, command, func() error {
			return operations.Rmdir(context.Background(), fdst, "")
		})
	},
}
