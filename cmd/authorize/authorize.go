// Package authorize provides the authorize command.
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
	template      string
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &noAutoBrowser, "auth-no-open-browser", "", false, "Do not automatically open auth link in default browser", "")
	flags.StringVarP(cmdFlags, &template, "template", "", "", "The path to a custom Go template for generating HTML responses", "")
}

var commandDefinition = &cobra.Command{
	Use:   "authorize <fs name> [base64_json_blob | client_id client_secret]",
	Short: `Remote authorization.`,
	Long: `Remote authorization. Used to authorize a remote or headless
rclone from a machine with a browser - use as instructed by
rclone config.

The command requires 1-3 arguments:
  - fs name (e.g., "drive", "s3", etc.)
  - Either a base64 encoded JSON blob obtained from a previous rclone config session
  - Or a client_id and client_secret pair obtained from the remote service

Use --auth-no-open-browser to prevent rclone to open auth
link in default browser automatically.

Use --template to generate HTML output via a custom Go template. If a blank string is provided as an argument to this flag, the default template is used.`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.27",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 3, command, args)
		return config.Authorize(context.Background(), args, noAutoBrowser, template)
	},
}
