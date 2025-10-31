// Package purge provides the purge command.
package purge

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "purge remote:path",
	Short: `Remove the path and all of its contents.`,
	Long: `Remove the path and all of its contents.  Note that this does not obey
include/exclude filters - everything will be removed.  Use the
[delete](/commands/rclone_delete/) command if you want to selectively
delete files. To delete empty directories only, use command
[rmdir](/commands/rclone_rmdir/) or [rmdirs](/commands/rclone_rmdirs/).

The concurrency of this operation is controlled by the ` + "`--checkers`" + ` global flag.
However, some backends will implement this command directly, in which
case ` + "`--checkers`" + ` will be ignored.

**Important**: Since this can cause data loss, test first with the
` + "`--dry-run` or the `--interactive`/`-i`" + ` flag.`,
	Annotations: map[string]string{
		"groups": "Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDir(args)
		cmd.Run(true, false, command, func() error {
			ctx := context.Background()
			fi := filter.GetConfig(ctx)
			if !fi.InActive() {
				fs.Fatalf(nil, "filters are not supported with purge (purge will delete everything unconditionally)")
			}
			return operations.Purge(context.Background(), fdst, "")
		})
	},
}
