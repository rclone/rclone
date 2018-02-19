package about

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "about remote:",
	Short: `Get quota information from the remote.`,
	Long: `
Get quota information from the remote, like bytes used/free/quota and bytes
used in the trash. Not supported by all remotes.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(true, false, command, func() error {
			return operations.About(fsrc)
		})
	},
}
