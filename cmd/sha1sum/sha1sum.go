package sha1sum

import (
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "sha1sum remote:path",
	Short: `Produces an sha1sum file for all the objects in the path.`,
	Long: `
Produces an sha1sum file for all the objects in the path.  This
is in the same format as the standard sha1sum tool produces.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return fs.Sha1sum(fsrc, os.Stdout)
		})
	},
}
