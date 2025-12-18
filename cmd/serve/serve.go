// Package serve provides the serve command.
package serve

import (
	"errors"

	"github.com/rclone/rclone/cmd"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(Command)
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "serve <protocol> [opts] <remote>",
	Short: `Serve a remote over a protocol.`,
	Long: `Serve a remote over a given protocol. Requires the use of a
subcommand to specify the protocol, e.g.

` + "```sh" + `
rclone serve http remote:
` + "```" + `

Each subcommand has its own options which you can see in their help.`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.39",
	},
	RunE: func(command *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("serve requires a protocol, e.g. 'rclone serve http remote:'")
		}
		return errors.New("unknown protocol")
	},
}
