package lsd

import (
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(lsdCmd)
}

var lsdCmd = &cobra.Command{
	Use:   "lsd remote:path",
	Short: `List all directories/containers/buckets in the the path.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, command, func() error {
			return fs.ListDir(fsrc, os.Stdout)
		})
	},
}
