package cat

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
	Use:   "cat remote:path",
	Short: `Concatenates any files and sends them to stdout.`,
	Long: `
rclone cat sends any files to standard output.

You can use it like this to output a single file

    rclone cat remote:path/to/file

Or like this to output any file in dir or subdirectories.

    rclone cat remote:path/to/dir

Or like this to output any .txt files in dir or subdirectories.

    rclone --include "*.txt" cat remote:path/to/dir
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return fs.Cat(fsrc, os.Stdout)
		})
	},
}
