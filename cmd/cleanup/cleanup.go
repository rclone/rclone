package cleanup

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(cleanupCmd)
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup remote:path",
	Short: `Clean up the remote if possible`,
	Long: `
Clean up the remote if possible.  Empty the trash or delete old file
versions. Not supported by all remotes.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(true, command, func() error {
			return fs.CleanUp(fsrc)
		})
	},
}
