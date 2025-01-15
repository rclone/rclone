package genautocomplete

import (
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	completionDefinition.AddCommand(fishCommandDefinition)
}

var fishCommandDefinition = &cobra.Command{
	Use:   "fish [output_file]",
	Short: `Output fish completion script for rclone.`,
	Long: `Generates a fish autocompletion script for rclone.

This writes to /etc/fish/completions/rclone.fish by default so will
probably need to be run with sudo or as root, e.g.

    sudo rclone completion fish

Logout and login again to use the autocompletion scripts, or source
them directly

    . /etc/fish/completions/rclone.fish

If you supply a command line argument the script will be written
there.

If output_file is "-", then the output will be written to stdout.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		out := "/etc/fish/completions/rclone.fish"
		if len(args) > 0 {
			if args[0] == "-" {
				err := cmd.Root.GenFishCompletion(os.Stdout, true)
				if err != nil {
					fs.Fatal(nil, fmt.Sprint(err))
				}
				return
			}
			out = args[0]
		}
		err := cmd.Root.GenFishCompletionFile(out, true)
		if err != nil {
			fs.Fatal(nil, fmt.Sprint(err))
		}
	},
}
