package link

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	expire = fs.DurationOff
	unlink = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.FVarP(cmdFlags, &expire, "expire", "", "The amount of time that the link will be valid")
	flags.BoolVarP(cmdFlags, &unlink, "unlink", "", unlink, "Remove existing public link to file/folder")
}

var commandDefinition = &cobra.Command{
	Use:   "link remote:path",
	Short: `Generate public link to file/folder.`,
	Long: `rclone link will create, retrieve or remove a public link to the given
file or folder.

    rclone link remote:path/to/file
    rclone link remote:path/to/folder/
    rclone link --unlink remote:path/to/folder/
    rclone link --expire 1d remote:path/to/file

If you supply the --expire flag, it will set the expiration time
otherwise it will use the default (100 years). **Note** not all
backends support the --expire flag - if the backend doesn't support it
then the link returned won't expire.

Use the --unlink flag to remove existing public links to the file or
folder. **Note** not all backends support "--unlink" flag - those that
don't will just ignore it.

If successful, the last line of the output will contain the
link. Exact capabilities depend on the remote, but the link will
always by default be created with the least constraints – e.g. no
expiry, no password protection, accessible without account.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc, remote := cmd.NewFsFile(args[0])
		cmd.Run(false, false, command, func() error {
			link, err := operations.PublicLink(context.Background(), fsrc, remote, expire, unlink)
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
