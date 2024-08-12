// Package sha1sum provides the sha1sum command.
package sha1sum

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
	Use:   "sha1sum remote:path",
	Short: `Produces an sha1sum file for all the objects in the path.`,
	Long: `Produces an sha1sum file for all the objects in the path.  This
is in the same format as the standard sha1sum tool produces.

By default, the hash is requested from the remote.  If SHA-1 is
not supported by the remote, no hash will be returned.  With the
download flag, the file will be downloaded from the remote and
hashed locally enabling SHA-1 for any remote.

For other algorithms, see the [hashsum](/commands/rclone_hashsum/)
command. Running ` + "`rclone sha1sum remote:path`" + ` is equivalent
to running ` + "`rclone hashsum SHA1 remote:path`" + `.

This command can also hash data received on standard input (stdin),
by not passing a remote:path, or by passing a hyphen as remote:path
when there is data to read (if not, the hyphen will be treated literally,
as a relative path).

This command can also hash data received on STDIN, if not passing
a remote:path.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.27",
		"groups":            "Filter,Listing",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 1, command, args)
		if found, err := hashsum.CreateFromStdinArg(hash.SHA1, args, 0); found {
			return err
		}
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			if hashsum.ChecksumFile != "" {
				fsum, sumFile := cmd.NewFsFile(hashsum.ChecksumFile)
				return operations.CheckSum(context.Background(), fsrc, fsum, sumFile, hash.SHA1, nil, hashsum.DownloadFlag)
			}
			if hashsum.HashsumOutfile == "" {
				return operations.HashLister(context.Background(), hash.SHA1, hashsum.OutputBase64, hashsum.DownloadFlag, fsrc, nil)
			}
			output, close, err := hashsum.GetHashsumOutput(hashsum.HashsumOutfile)
			if err != nil {
				return err
			}
			defer close()
			return operations.HashLister(context.Background(), hash.SHA1, hashsum.OutputBase64, hashsum.DownloadFlag, fsrc, output)
		})
		return nil
	},
}
