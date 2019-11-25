package copyurl

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	autoFilename = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &autoFilename, "auto-filename", "a", autoFilename, "Get the file name from the url and use it for destination file path")
}

var commandDefinition = &cobra.Command{
	Use:   "copyurl https://example.com dest:path",
	Short: `Copy url content to dest.`,
	Long: `
Download urls content and copy it to destination 
without saving it in tmp storage.

Setting --auto-filename flag will cause retrieving file name from url and using it in destination path. 
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)

		var dstFileName string
		var fsdst fs.Fs
		if autoFilename {
			fsdst = cmd.NewFsDir(args[1:])
		} else {
			fsdst, dstFileName = cmd.NewFsDstFile(args[1:])
		}

		cmd.Run(true, true, command, func() error {
			_, err := operations.CopyURL(context.Background(), fsdst, dstFileName, args[0], autoFilename)
			return err
		})
	},
}
