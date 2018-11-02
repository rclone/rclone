package copyurl

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "copyurl https://example.com dest:path",
	Short: `Copy url content to dest.`,
	Long: `
Download urls content and copy it to destination 
without saving it in tmp storage.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsdst, dstFileName := cmd.NewFsDstFile(args[1:])

		cmd.Run(true, true, command, func() error {
			_, err := operations.CopyURL(fsdst, dstFileName, args[0])
			return err
		})
	},
}
