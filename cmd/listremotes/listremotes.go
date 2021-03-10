package ls

import (
	"fmt"
	"sort"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/cobra"
)

// Globals
var (
	listLong bool
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &listLong, "long", "", listLong, "Show the type as well as names.")
}

var commandDefinition = &cobra.Command{
	Use:   "listremotes",
	Short: `List all the remotes in the config file.`,
	Long: `
rclone listremotes lists all the available remotes from the config file.

When uses with the -l flag it lists the types too.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		remotes := config.FileSections()
		sort.Strings(remotes)
		maxlen := 1
		for _, remote := range remotes {
			if len(remote) > maxlen {
				maxlen = len(remote)
			}
		}
		for _, remote := range remotes {
			if listLong {
				remoteType := config.FileGet(remote, "type")
				fmt.Printf("%-*s %s\n", maxlen+1, remote+":", remoteType)
			} else {
				fmt.Printf("%s:\n", remote)
			}
		}
	},
}
