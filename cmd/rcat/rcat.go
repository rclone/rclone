package rcat

import (
	"log"
	"os"
	"time"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "rcat remote:path",
	Short: `Copies standard input to file on remote.`,
	Long: `
rclone rcat reads from standard input (stdin) and copies it to a
single remote file.

    echo "hello world" | rclone rcat remote:path/to/file

Note that since the size is not known in advance, chunking options
will likely be ignored. The upload can also not be retried because
the data is not kept around until the upload succeeds. If you need
to transfer a lot of data, you're better off caching locally and
then ` + "`rclone move`" + ` it to the destination.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)

		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			log.Fatalf("nothing to read from standard input (stdin).")
		}

		fdst, dstFileName := cmd.NewFsDstFile(args)
		cmd.Run(false, false, command, func() error {
			return fs.Rcat(fdst, dstFileName, os.Stdin, time.Now())
		})
	},
}
