package authorize

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/cobra"
)

var (
	noAutoBrowser bool
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &noAutoBrowser, "auth-no-open-browser", "", false, "Do not automatically open auth link in default browser")
}

var commandDefinition = &cobra.Command{
	Use:   "authorize",
	Short: `Remote authorization.`,
	Long: `
Remote authorization. Used to authorize a remote or headless
rclone from a machine with a browser - use as instructed by
rclone config.

Use the --auth-no-open-browser to prevent rclone to open auth
link in default browser automatically.`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 3, command, args)
		return config.Authorize(context.Background(), args, noAutoBrowser)
	},
}
