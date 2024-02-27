// Package ls provides the ls command.
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
	flags.BoolVarP(cmdFlags, &listLong, "long", "", listLong, "Show the type and the description as well as names", "")
}

var commandDefinition = &cobra.Command{
	Use:   "listremotes",
	Short: `List all the remotes in the config file and defined in environment variables.`,
	Long: `
rclone listremotes lists all the available remotes from the config file.

When used with the ` + "`--long`" + ` flag it lists the types and the descriptions too.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.34",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		remotes := config.FileSections()
		sort.Strings(remotes)
		maxlen := 1
		maxlentype := 1
		for _, remote := range remotes {
			if len(remote) > maxlen {
				maxlen = len(remote)
			}
			t := config.FileGet(remote, "type")
			if len(t) > maxlentype {
				maxlentype = len(t)
			}
		}
		for _, remote := range remotes {
			if listLong {
				remoteType := config.FileGet(remote, "type")
				description := config.FileGet(remote, "description")
				fmt.Printf("%-*s %-*s %s\n", maxlen+1, remote+":", maxlentype+1, remoteType, description)
			} else {
				fmt.Printf("%s:\n", remote)
			}
		}
	},
}
