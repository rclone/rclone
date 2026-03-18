//go:build !plan9

// Package archive implements 'rclone archive'.
package archive

import (
	"errors"

	"github.com/rclone/rclone/cmd"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(Command)
}

// Command - archive command
var Command = &cobra.Command{
	Use:   "archive <action> [opts] <source> [<destination>]",
	Short: `Perform an action on an archive.`,
	Long: `Perform an action on an archive. Requires the use of a
subcommand to specify the protocol, e.g.

    rclone archive list remote:file.zip

Each subcommand has its own options which you can see in their help.

See [rclone archive create](/commands/rclone_archive_create/) for the
archive formats supported.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.72",
	},
	RunE: func(command *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("archive requires an action, e.g. 'rclone archive list remote:'")
		}
		return errors.New("unknown action")
	},
}
