// Package serve provides the serve command.
package serve

import (
	"errors"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/dlna"
	"github.com/rclone/rclone/cmd/serve/docker"
	"github.com/rclone/rclone/cmd/serve/ftp"
	"github.com/rclone/rclone/cmd/serve/http"
	"github.com/rclone/rclone/cmd/serve/nfs"
	"github.com/rclone/rclone/cmd/serve/restic"
	"github.com/rclone/rclone/cmd/serve/s3"
	"github.com/rclone/rclone/cmd/serve/sftp"
	"github.com/rclone/rclone/cmd/serve/webdav"
	"github.com/spf13/cobra"
)

func init() {
	Command.AddCommand(http.Command)
	if webdav.Command != nil {
		Command.AddCommand(webdav.Command)
	}
	if restic.Command != nil {
		Command.AddCommand(restic.Command)
	}
	if dlna.Command != nil {
		Command.AddCommand(dlna.Command)
	}
	if ftp.Command != nil {
		Command.AddCommand(ftp.Command)
	}
	if sftp.Command != nil {
		Command.AddCommand(sftp.Command)
	}
	if docker.Command != nil {
		Command.AddCommand(docker.Command)
	}
	if nfs.Command != nil {
		Command.AddCommand(nfs.Command)
	}
	if s3.Command != nil {
		Command.AddCommand(s3.Command)
	}
	cmd.Root.AddCommand(Command)
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "serve <protocol> [opts] <remote>",
	Short: `Serve a remote over a protocol.`,
	Long: `Serve a remote over a given protocol. Requires the use of a
subcommand to specify the protocol, e.g.

    rclone serve http remote:

Each subcommand has its own options which you can see in their help.
`,
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
