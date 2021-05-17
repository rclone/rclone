package hashsum

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Global hashsum flags for reuse in hashsum, md5sum, sha1sum
var (
	OutputBase64   = false
	DownloadFlag   = false
	HashsumOutfile = ""
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	AddHashFlags(cmdFlags)
}

// AddHashFlags is a convenience function to add the command flags OutputBase64 and DownloadFlag to hashsum, md5sum, sha1sum
func AddHashFlags(cmdFlags *pflag.FlagSet) {
	flags.BoolVarP(cmdFlags, &OutputBase64, "base64", "", OutputBase64, "Output base64 encoded hashsum")
	flags.StringVarP(cmdFlags, &HashsumOutfile, "output-file", "", HashsumOutfile, "Output hashsums to a file rather than the terminal")
	flags.BoolVarP(cmdFlags, &DownloadFlag, "download", "", DownloadFlag, "Download the file and hash it locally; if this flag is not specified, the hash is requested from the remote")
}

// GetHashsumOutput opens and closes the output file when using the output-file flag
func GetHashsumOutput(filename string) (out *os.File, close func(), err error) {
	out, err = os.Create(filename)
	if err != nil {
		err = errors.Wrapf(err, "Failed to open output file %v", filename)
		return nil, nil, err
	}

	close = func() {
		err := out.Close()
		if err != nil {
			fs.Errorf(nil, "Failed to close output file %v: %v", filename, err)
		}
	}

	return out, close, nil
}

var commandDefinition = &cobra.Command{
	Use:   "hashsum <hash> remote:path",
	Short: `Produces a hashsum file for all the objects in the path.`,
	Long: `
Produces a hash file for all the objects in the path using the hash
named.  The output is in the same format as the standard
md5sum/sha1sum tool.

By default, the hash is requested from the remote.  If the hash is
not supported by the remote, no hash will be returned.  With the
download flag, the file will be downloaded from the remote and
hashed locally enabling any hash for any remote.

Run without a hash to see the list of all supported hashes, e.g.

    $ rclone hashsum
    Supported hashes are:
      * MD5
      * SHA-1
      * DropboxHash
      * QuickXorHash

Then

    $ rclone hashsum MD5 remote:path
`,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 2, command, args)
		if len(args) == 0 {
			fmt.Printf("Supported hashes are:\n")
			for _, ht := range hash.Supported().Array() {
				fmt.Printf("  * %v\n", ht)
			}
			return nil
		} else if len(args) == 1 {
			return errors.New("need hash type and remote")
		}
		var ht hash.Type
		err := ht.Set(args[0])
		if err != nil {
			return err
		}
		fsrc := cmd.NewFsSrc(args[1:])

		cmd.Run(false, false, command, func() error {
			if HashsumOutfile == "" {
				return operations.HashLister(context.Background(), ht, OutputBase64, DownloadFlag, fsrc, nil)
			}
			output, close, err := GetHashsumOutput(HashsumOutfile)
			if err != nil {
				return err
			}
			defer close()
			return operations.HashLister(context.Background(), ht, OutputBase64, DownloadFlag, fsrc, output)
		})
		return nil
	},
}
