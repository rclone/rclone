package check

import (
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "check source:path dest:path",
	Short: `Checks the files in the source and destination match.`,
	Long: `
Checks the files in the source and destination match.  It
compares sizes and MD5SUMs and prints a report of files which
don't match.  It doesn't alter the source or destination.

` + "`" + `--size-only` + "`" + ` may be used to only compare the sizes, not the MD5SUMs.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, fdst := cmd.NewFsSrcDst(args)
		cmd.Run(false, false, command, func() error {
			return fs.Check(fdst, fsrc)
		})
	},
}
