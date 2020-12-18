package dbhashsum

import (
	"context"

	"github.com/rclone/rclone/backend/dropbox"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/hashsum"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	hashsum.AddHashFlags(cmdFlags)
}

var commandDefinition = &cobra.Command{
	Use:   "dbhashsum remote:path",
	Short: `Produces a Dropbox hash file for all the objects in the path.`,
	Long: `
Produces a Dropbox hash file for all the objects in the path.  The
hashes are calculated according to [Dropbox content hash
rules](https://www.dropbox.com/developers/reference/content-hash).
The output is in the same format as md5sum and sha1sum.

By default, the hash is requested from the remote.  If Dropbox hash is
not supported by the remote, no hash will be returned.  With the
download flag, the file will be downloaded from the remote and
hashed locally enabling Dropbox hash for any remote.
`,
	Hidden: true,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		fs.Logf(nil, `"rclone dbhashsum" is deprecated, use "rclone hashsum %v %s" instead`, dropbox.DbHashType, args[0])
		cmd.Run(false, false, command, func() error {
			if hashsum.HashsumOutfile == "" {
				return operations.HashLister(context.Background(), dropbox.DbHashType, hashsum.OutputBase64, hashsum.DownloadFlag, fsrc, nil)
			}
			output, close, err := hashsum.GetHashsumOutput(hashsum.HashsumOutfile)
			if err != nil {
				return err
			}
			defer close()
			return operations.HashLister(context.Background(), dropbox.DbHashType, hashsum.OutputBase64, hashsum.DownloadFlag, fsrc, output)
		})
	},
}
