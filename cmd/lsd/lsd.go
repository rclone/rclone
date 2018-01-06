package lsd

import (
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/ls/lshelp"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "lsd remote:path",
	Short: `List all directories/containers/buckets in the path.`,
	Long: `
Lists the directories in the source path to standard output. Recurses
by default.
` + lshelp.Help,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return fs.ListDir(fsrc, os.Stdout)
		})
	},
}
