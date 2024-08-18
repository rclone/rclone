// Package hashsum provides the hashsum command.
package hashsum

import (
	"context"
	"fmt"
	"os"

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
	ChecksumFile   = ""
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	AddHashsumFlags(cmdFlags)
}

// AddHashsumFlags is a convenience function to add the command flags OutputBase64 and DownloadFlag to hashsum, md5sum, sha1sum
func AddHashsumFlags(cmdFlags *pflag.FlagSet) {
	flags.BoolVarP(cmdFlags, &OutputBase64, "base64", "", OutputBase64, "Output base64 encoded hashsum", "")
	flags.StringVarP(cmdFlags, &HashsumOutfile, "output-file", "", HashsumOutfile, "Output hashsums to a file rather than the terminal", "")
	flags.StringVarP(cmdFlags, &ChecksumFile, "checkfile", "C", ChecksumFile, "Validate hashes against a given SUM file instead of printing them", "")
	flags.BoolVarP(cmdFlags, &DownloadFlag, "download", "", DownloadFlag, "Download the file and hash it locally; if this flag is not specified, the hash is requested from the remote", "")
}

// GetHashsumOutput opens and closes the output file when using the output-file flag
func GetHashsumOutput(filename string) (out *os.File, close func(), err error) {
	out, err = os.Create(filename)
	if err != nil {
		err = fmt.Errorf("failed to open output file %v: %w", filename, err)
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

// CreateFromStdinArg checks args and produces hashsum from standard input if it is requested
func CreateFromStdinArg(ht hash.Type, args []string, startArg int) (bool, error) {
	var stdinArg bool
	if len(args) == startArg {
		// Missing arg: Always read from stdin
		stdinArg = true
	} else if len(args) > startArg && args[startArg] == "-" {
		// Special arg: Read from stdin only if there is data available
		if fi, _ := os.Stdin.Stat(); fi.Mode()&os.ModeCharDevice == 0 {
			stdinArg = true
		}
	}
	if !stdinArg {
		return false, nil
	}
	if HashsumOutfile == "" {
		return true, operations.HashSumStream(ht, OutputBase64, os.Stdin, nil)
	}
	output, close, err := GetHashsumOutput(HashsumOutfile)
	if err != nil {
		return true, err
	}
	defer close()
	return true, operations.HashSumStream(ht, OutputBase64, os.Stdin, output)
}

var commandDefinition = &cobra.Command{
	Use:   "hashsum [<hash> remote:path]",
	Short: `Produces a hashsum file for all the objects in the path.`,
	Long: `Produces a hash file for all the objects in the path using the hash
named.  The output is in the same format as the standard
md5sum/sha1sum tool.

By default, the hash is requested from the remote.  If the hash is
not supported by the remote, no hash will be returned.  With the
download flag, the file will be downloaded from the remote and
hashed locally enabling any hash for any remote.

For the MD5 and SHA1 algorithms there are also dedicated commands,
[md5sum](/commands/rclone_md5sum/) and [sha1sum](/commands/rclone_sha1sum/).

This command can also hash data received on standard input (stdin),
by not passing a remote:path, or by passing a hyphen as remote:path
when there is data to read (if not, the hyphen will be treated literally,
as a relative path).

Run without a hash to see the list of all supported hashes, e.g.

    $ rclone hashsum
` + hash.HelpString(4) + `
Then

    $ rclone hashsum MD5 remote:path

Note that hash names are case insensitive and values are output in lower case.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.41",
		"groups":            "Filter,Listing",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 2, command, args)
		if len(args) == 0 {
			fmt.Print(hash.HelpString(0))
			return nil
		}
		var ht hash.Type
		err := ht.Set(args[0])
		if err != nil {
			fmt.Println(hash.HelpString(0))
			return err
		}
		if found, err := CreateFromStdinArg(ht, args, 1); found {
			return err
		}
		fsrc := cmd.NewFsSrc(args[1:])
		cmd.Run(false, false, command, func() error {
			if ChecksumFile != "" {
				fsum, sumFile := cmd.NewFsFile(ChecksumFile)
				return operations.CheckSum(context.Background(), fsrc, fsum, sumFile, ht, nil, DownloadFlag)
			}
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
