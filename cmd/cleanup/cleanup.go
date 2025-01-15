// Package cleanup provides the cleanup command.
package cleanup

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "cleanup remote:path",
	Short: `Clean up the remote if possible.`,
	Long: `Clean up the remote if possible.  Empty the trash or delete old file
versions. Not supported by all remotes.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.31",
		"groups":            "Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(true, false, command, func() error {
			return operations.CleanUp(context.Background(), fsrc)
		})
	},
}
