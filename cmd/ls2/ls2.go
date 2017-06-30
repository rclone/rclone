package ls2

import (
	"fmt"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

var (
	recurse bool
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().BoolVarP(&recurse, "recursive", "R", false, "Recurse into the listing.")
}

var commandDefintion = &cobra.Command{
	Use:    "ls2 remote:path",
	Short:  `List directories and objects in the path.`,
	Hidden: true,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return fs.Walk(fsrc, "", false, fs.ConfigMaxDepth(recurse), func(path string, entries fs.DirEntries, err error) error {
				if err != nil {
					fs.Stats.Error()
					fs.Errorf(path, "error listing: %v", err)
					return nil
				}
				for _, entry := range entries {
					_, isDir := entry.(fs.Directory)
					if isDir {
						fmt.Println(entry.Remote() + "/")
					} else {
						fmt.Println(entry.Remote())
					}
				}
				return nil
			})
		})
	},
}
