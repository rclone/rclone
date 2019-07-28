package authorize

import (
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "authorize",
	Short: `Remote authorization.`,
	Long: `
Remote authorization. Used to authorize a remote or headless
rclone from a machine with a browser - use as instructed by
rclone config.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 3, command, args)
		config.Authorize(args)
	},
}
