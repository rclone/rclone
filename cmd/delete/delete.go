package delete

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "delete remote:path",
	Short: `Remove the contents of path.`,
	Long: `
Remove the files in path.  Unlike ` + "`" + `purge` + "`" + ` it obeys include/exclude
filters so can be used to selectively delete files.

` + "`" + `rclone delete` + "`" + ` only deletes objects but leaves the directory structure
alone. If you want to delete a directory and all of its contents use
` + "`" + `rclone purge` + "`" + `

Eg delete all files bigger than 100MBytes

Check what would be deleted first (use either)

    rclone --min-size 100M lsl remote:path
    rclone --dry-run --min-size 100M delete remote:path

Then delete

    rclone --min-size 100M delete remote:path

That reads "delete everything with a minimum size of 100 MB", hence
delete all files bigger than 100MBytes.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(true, false, command, func() error {
			return operations.Delete(context.Background(), fsrc)
		})
	},
}
