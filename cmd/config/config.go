package config

import (
	"fmt"
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "config [function]",
	Short: `Enter an interactive configuration session.`,
	Long: "`rclone config`" + `
 enters an interactive configuration sessions where you can setup
new remotes and manage existing ones. You may also set or remove a password to
protect your configuration.

Additional functions:

  * ` + "`rclone config edit`" + ` – same as above
  * ` + "`rclone config file`" + ` – show path of configuration file in use
  * ` + "`rclone config show`" + ` – print (decrypted) config file
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		if len(args) == 0 {
			fs.EditConfig()
			return
		}

		switch args[0] {
		case "edit":
			fs.EditConfig()
		case "show":
			fs.ShowConfig()
		case "file":
			fs.ShowConfigLocation()
		default:
			fmt.Fprintf(os.Stderr, "Unknown subcommand %q, %s only supports edit, show and file.\n", args[0], command.Name())
		}
	},
}
