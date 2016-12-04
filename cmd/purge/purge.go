package purge

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "purge remote:path",
	Short: `Remove the path and all of its contents.`,
	Long: `
Remove the path and all of its contents.  Note that this does not obey
include/exclude filters - everything will be removed.  Use ` + "`" + `delete` + "`" + ` if
you want to selectively delete files.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDst(args)
		cmd.Run(true, false, command, func() error {
			return fs.Purge(fdst)
		})
	},
}
