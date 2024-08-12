// Package md5sum provides the md5sum command.
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
	hashsum.AddHashsumFlags(cmdFlags)
}

var commandDefinition = &cobra.Command{
	Use:   "md5sum remote:path",
	Short: `Produces an md5sum file for all the objects in the path.`,
	Long: `Produces an md5sum file for all the objects in the path.  This
is in the same format as the standard md5sum tool produces.

By default, the hash is requested from the remote.  If MD5 is
not supported by the remote, no hash will be returned.  With the
download flag, the file will be downloaded from the remote and
hashed locally enabling MD5 for any remote.

For other algorithms, see the [hashsum](/commands/rclone_hashsum/)
command. Running ` + "`rclone md5sum remote:path`" + ` is equivalent
to running ` + "`rclone hashsum MD5 remote:path`" + `.

This command can also hash data received on standard input (stdin),
by not passing a remote:path, or by passing a hyphen as remote:path
when there is data to read (if not, the hyphen will be treated literally,
as a relative path).
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.02",
		"groups":            "Filter,Listing",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 1, command, args)
		if found, err := hashsum.CreateFromStdinArg(hash.MD5, args, 0); found {
			return err
		}
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			if hashsum.ChecksumFile != "" {
				fsum, sumFile := cmd.NewFsFile(hashsum.ChecksumFile)
				return operations.CheckSum(context.Background(), fsrc, fsum, sumFile, hash.MD5, nil, hashsum.DownloadFlag)
			}
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
		return nil
	},
}
