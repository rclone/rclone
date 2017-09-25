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
  * ` + "`rclone config listproviders`" + ` – List, in json format, the protocols supported by sync
  * ` + "`rclone config optionsprovider type`" + ` – Lists all the options needed to connect to a protocol
  * ` + "`rclone config jsonconfig`" + ` – print (decrypted) config file
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 4, command, args)
		if len(args) == 0 {
			fs.EditConfig()
		} else if (len(args) == 1) {
			switch args[0] {
			case "edit":
				fs.EditConfig()
			case "show":
				fs.ShowConfig()
			case "file":
				fs.ShowConfigLocation()
			case "listproviders":
				fs.ListProviders()
			default:
				fmt.Fprintf(os.Stderr, "Unknown subcommand %q, %s only supports edit, show and file.\n", args[0], command.Name())
			}
		} else if (len(args) == 2) {
			if ((args[0] == "listoptions") && (args[1] != "")) {
				fs.ListOptions(args[1])
			} else {
				fmt.Fprintf(os.Stderr, "Unknown subcommand %q %q, %s only supports optionsprovider <type>.\n", args[0], args[1], command.Name())
			}
		} else if (len(args) == 4) {
			if ((args[0] == "jsonconfig") && (args[1] != "") && (args[2] != "") && (args[3]!= "")) {
				fs.JsonConfig(args[1], args[2], args[3])
			} else {
				fmt.Fprintf(os.Stderr, "Unknown subcommand %q %q %q %q, %s only supports jsonconfig <name> <type> <json:options>.\n", args[0], args[1], args[2], args[3], command.Name())
		    }
		}
		return
	},
}
