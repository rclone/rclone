package lsl

import (
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "lsl remote:path",
	Short: `List all the objects path with modification time, size and path.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return fs.ListLong(fsrc, os.Stdout)
		})
	},
}
