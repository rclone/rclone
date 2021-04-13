package rcat

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	size = int64(-1)
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.Int64VarP(cmdFlags, &size, "size", "", size, "File size hint to preallocate")
}

var commandDefinition = &cobra.Command{
	Use:   "rcat remote:path",
	Short: `Copies standard input to file on remote.`,
	Long: `
rclone rcat reads from standard input (stdin) and copies it to a
single remote file.

    echo "hello world" | rclone rcat remote:path/to/file
    ffmpeg - | rclone rcat remote:path/to/file

If the remote file already exists, it will be overwritten.

rcat will try to upload small files in a single request, which is
usually more efficient than the streaming/chunked upload endpoints,
which use multiple requests. Exact behaviour depends on the remote.
What is considered a small file may be set through
` + "`--streaming-upload-cutoff`" + `. Uploading only starts after
the cutoff is reached or if the file ends before that. The data
must fit into RAM. The cutoff needs to be small enough to adhere
the limits of your remote, please see there. Generally speaking,
setting this cutoff too high will decrease your performance.

Use the |--size| flag to preallocate the file in advance at the remote end
and actually stream it, even if remote backend doesn't support streaming.

|--size| should be the exact size of the input stream in bytes. If the
size of the stream is different in length to the |--size| passed in
then the transfer will likely fail.

Note that the upload can also not be retried because the data is
not kept around until the upload succeeds. If you need to transfer
a lot of data, you're better off caching locally and then
` + "`rclone move`" + ` it to the destination.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)

		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			log.Fatalf("nothing to read from standard input (stdin).")
		}

		fdst, dstFileName := cmd.NewFsDstFile(args)
		cmd.Run(false, false, command, func() error {
			_, err := operations.RcatSize(context.Background(), fdst, dstFileName, os.Stdin, size, time.Now())
			return err
		})
	},
}
