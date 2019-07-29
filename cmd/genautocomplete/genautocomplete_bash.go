package genautocomplete

import (
	"log"

	"github.com/rclone/rclone/cmd"
	"github.com/spf13/cobra"
)

func init() {
	completionDefinition.AddCommand(bashCommandDefinition)
}

var bashCommandDefinition = &cobra.Command{
	Use:   "bash [output_file]",
	Short: `Output bash completion script for rclone.`,
	Long: `
Generates a bash shell autocompletion script for rclone.

This writes to /etc/bash_completion.d/rclone by default so will
probably need to be run with sudo or as root, eg

    sudo rclone genautocomplete bash

Logout and login again to use the autocompletion scripts, or source
them directly

    . /etc/bash_completion

If you supply a command line argument the script will be written
there.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		out := "/etc/bash_completion.d/rclone"
		if len(args) > 0 {
			out = args[0]
		}
		err := cmd.Root.GenBashCompletionFile(out)
		if err != nil {
			log.Fatal(err)
		}
	},
}
