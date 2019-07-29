package genautocomplete

import (
	"log"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/spf13/cobra"
)

func init() {
	completionDefinition.AddCommand(zshCommandDefinition)
}

var zshCommandDefinition = &cobra.Command{
	Use:   "zsh [output_file]",
	Short: `Output zsh completion script for rclone.`,
	Long: `
Generates a zsh autocompletion script for rclone.

This writes to /usr/share/zsh/vendor-completions/_rclone by default so will
probably need to be run with sudo or as root, eg

    sudo rclone genautocomplete zsh

Logout and login again to use the autocompletion scripts, or source
them directly

    autoload -U compinit && compinit

If you supply a command line argument the script will be written
there.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		out := "/usr/share/zsh/vendor-completions/_rclone"
		if len(args) > 0 {
			out = args[0]
		}
		outFile, err := os.Create(out)
		if err != nil {
			log.Fatal(err)
		}
		defer func() { _ = outFile.Close() }()
		err = cmd.Root.GenZshCompletion(outFile)
		if err != nil {
			log.Fatal(err)
		}
	},
}
