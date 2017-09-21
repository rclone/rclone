package config

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "manualconfig",
	Short: `List all options of provider.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(3, 3, command, args)
		fs.SetFsNewRemote(args[0], args[1], args[2])
	},
}
