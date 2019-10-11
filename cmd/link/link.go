package link

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "link remote:path",
	Short: `Generate public link to file/folder.`,
	Long: `
rclone link will create or retrieve a public link to the given file or folder.

    rclone link remote:path/to/file
    rclone link remote:path/to/folder/

If successful, the last line of the output will contain the link. Exact
capabilities depend on the remote, but the link will always be created with
the least constraints – e.g. no expiry, no password protection, accessible
without account.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc, remote := cmd.NewFsFile(args[0])
		cmd.Run(false, false, command, func() error {
			link, err := operations.PublicLink(context.Background(), fsrc, remote)
			if err != nil {
				return err
			}
			if link != "" {
				fmt.Println(link)
			}
			return nil
		})
	},
}
