// Package rmdir provides the rmdir command.
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
	Long: `This recursively removes any empty directories (including directories
that only contain empty directories), that it finds under the path.
The root path itself will also be removed if it is empty, unless
you supply the ` + "`--leave-root`" + ` flag.

Use command [rmdir](/commands/rclone_rmdir/) to delete just the empty
directory given by path, not recurse.

This is useful for tidying up remotes that rclone has left a lot of
empty directories in. For example the [delete](/commands/rclone_delete/)
command will delete files but leave the directory structure (unless
used with option ` + "`--rmdirs`" + `).

This will delete ` + "`--checkers`" + ` directories concurrently so
if you have thousands of empty directories consider increasing this number.

To delete a path and any objects in it, use the [purge](/commands/rclone_purge/)
command.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.35",
		"groups":            "Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDir(args)
		cmd.Run(true, false, command, func() error {
			return operations.Rmdirs(context.Background(), fdst, "", leaveRoot)
		})
	},
}
