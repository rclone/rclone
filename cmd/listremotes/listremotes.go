package ls

import (
	"fmt"
	"sort"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

// Globals
var (
	listLong bool
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().BoolVarP(&listLong, "long", "l", listLong, "Show the type as well as names.")
}

var commandDefintion = &cobra.Command{
	Use:   "listremotes",
	Short: `List all the remotes in the config file.`,
	Long: `
rclone listremotes lists all the available remotes from the config file.

When uses with the -l flag it lists the types too.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		remotes := fs.ConfigFileSections()
		sort.Strings(remotes)
		maxlen := 1
		for _, remote := range remotes {
			if len(remote) > maxlen {
				maxlen = len(remote)
			}
		}
		for _, remote := range remotes {
			if listLong {
				remoteType := fs.ConfigFileGet(remote, "type", "UNKNOWN")
				fmt.Printf("%-*s %s\n", maxlen+1, remote+":", remoteType)
			} else {
				fmt.Printf("%s:\n", remote)
			}
		}
	},
}
