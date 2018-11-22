package copyurl

import (
	"net/http"
	"time"

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
			resp, err := http.Get(args[0])
			if err != nil {

				return err
			}

			_, err = operations.RcatSize(fsdst, dstFileName, resp.Body, resp.ContentLength, time.Now())

			return err
		})
	},
}
