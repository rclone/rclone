package copyurl

import (
	"net/http"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "copyurl remote:path",
	Short: `Stream remote file content.`,
	Long: `
Download urls content and copy it to destination.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsdst, dstFileName := cmd.NewFsDstFile(args[1:])

		cmd.Run(true, true, command, func() error {
			resp, err := http.Get(args[0])
			if err != nil {

				return err
			}

			return operations.UploadHttpBody(fsdst, resp.Body, resp.ContentLength, dstFileName)
		})
	},
}
