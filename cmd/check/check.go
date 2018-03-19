package check

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs/operations"
	"github.com/spf13/cobra"
)

// Globals
var (
	download = false
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().BoolVarP(&download, "download", "", download, "Check by downloading rather than with hash.")
}

var commandDefintion = &cobra.Command{
	Use:   "check source:path dest:path",
	Short: `Checks the files in the source and destination match.`,
	Long: `
Checks the files in the source and destination match.  It compares
sizes and hashes (MD5 or SHA1) and logs a report of files which don't
match.  It doesn't alter the source or destination.

If you supply the --size-only flag, it will only compare the sizes not
the hashes as well.  Use this for a quick check.

If you supply the --download flag, it will download the data from
both remotes and check them against each other on the fly.  This can
be useful for remotes that don't support hashes or if you really want
to check all the data.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, fdst := cmd.NewFsSrcDst(args)
		cmd.Run(false, false, command, func() error {
			if download {
				return operations.CheckDownload(fdst, fsrc)
			}
			return operations.Check(fdst, fsrc)
		})
	},
}
