package dbhashsum

import (
	"context"
	"os"

	"github.com/rclone/rclone/backend/dropbox"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "dbhashsum remote:path",
	Short: `Produces a Dropbox hash file for all the objects in the path.`,
	Long: `
Produces a Dropbox hash file for all the objects in the path.  The
hashes are calculated according to [Dropbox content hash
rules](https://www.dropbox.com/developers/reference/content-hash).
The output is in the same format as md5sum and sha1sum.
`,
	Hidden: true,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		fs.Logf(nil, `"rclone dbhashsum" is deprecated, use "rclone hashsum %v %s" instead`, dropbox.DbHashType, args[0])
		cmd.Run(false, false, command, func() error {
			return operations.HashLister(context.Background(), dropbox.DbHashType, fsrc, os.Stdout)
		})
	},
}
