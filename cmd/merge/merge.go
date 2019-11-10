package merge

import (
"github.com/rclone/rclone/cmd"
"github.com/spf13/cobra"
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
		cmd.Run(true, true, command, func() error {
			print("MERGE---------")
			print(fsrc.Name(), fdst.Name())
			print(srcFileName)
			//if srcFileName == "" {
			//return sync.Sync(context.Background(), fdst, fsrc, createEmptySrcDirs)
			// }
			//return operations.CopyFile(context.Background(), fdst, fsrc, srcFileName, srcFileName)
			return nil
		})
		return
	},
}
