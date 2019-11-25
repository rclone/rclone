package deletefile

import (
	"context"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "deletefile remote:path",
	Short: `Remove a single file from remote.`,
	Long: `
Remove a single file from remote.  Unlike ` + "`" + `delete` + "`" + ` it cannot be used to
remove a directory and it doesn't obey include/exclude filters - if the specified file exists,
it will always be removed.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fs, fileName := cmd.NewFsFile(args[0])
		cmd.Run(true, false, command, func() error {
			if fileName == "" {
				return errors.Errorf("%s is a directory or doesn't exist", args[0])
			}
			fileObj, err := fs.NewObject(context.Background(), fileName)
			if err != nil {
				return err
			}
			return operations.DeleteFile(context.Background(), fileObj)
		})
	},
}
