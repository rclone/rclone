package merge

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
)

var (
	// createEmptySrcDirs = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	// cmdFlags := commandDefinition.Flags()
	// flags.BoolVarP(cmdFlags, &createEmptySrcDirs, "create-empty-src-dirs", "", createEmptySrcDirs, "Create empty source dirs on destination after sync")
	return
}

var commandDefinition = &cobra.Command{
	Use:   "merge source:path dest:path",
	Short: `Make source and dest identical, by two way syncing.`,
	Long: `

`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
		cmd.Run(false, false, command, func() error {
			if srcFileName == "" {
				return operations.MergeFn(context.Background(), fdst, fsrc)
			}
			return errors.New("not supporting files")
		})
	},
}
