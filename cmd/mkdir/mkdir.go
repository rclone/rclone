package mkdir

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(mkdirCmd)
}

var mkdirCmd = &cobra.Command{
	Use:   "mkdir remote:path",
	Short: `Make the path if it doesn't already exist.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDst(args)
		cmd.Run(true, command, func() error {
			return fs.Mkdir(fdst)
		})
	},
}
