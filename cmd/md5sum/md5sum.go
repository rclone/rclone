package md5sum

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/hashsum"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	hashsum.AddHashFlags(cmdFlags)
}

var commandDefinition = &cobra.Command{
	Use:   "md5sum remote:path",
	Short: `Produces an md5sum file for all the objects in the path.`,
	Long: `
Produces an md5sum file for all the objects in the path.  This
is in the same format as the standard md5sum tool produces.

By default, the hash is requested from the remote.  If MD5 is
not supported by the remote, no hash will be returned.  With the
download flag, the file will be downloaded from the remote and
hashed locally enabling MD5 for any remote.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			if hashsum.HashsumOutfile == "" {
				return operations.HashLister(context.Background(), hash.MD5, hashsum.OutputBase64, hashsum.DownloadFlag, fsrc, nil)
			}
			output, close, err := hashsum.GetHashsumOutput(hashsum.HashsumOutfile)
			if err != nil {
				return err
			}
			defer close()
			return operations.HashLister(context.Background(), hash.MD5, hashsum.OutputBase64, hashsum.DownloadFlag, fsrc, output)
		})
	},
}
