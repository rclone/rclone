package genautocomplete

import (
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	completionDefinition.AddCommand(bashCommandDefinition)
}

var bashCommandDefinition = &cobra.Command{
	Use:   "bash [output_file]",
	Short: `Output bash completion script for rclone.`,
	Long: `Generates a bash shell autocompletion script for rclone.

By default, when run without any arguments, 

    rclone completion bash

the generated script will be written to

    /etc/bash_completion.d/rclone

and so rclone will probably need to be run as root, or with sudo.

If you supply a path to a file as the command line argument, then 
the generated script will be written to that file, in which case
you should not need root privileges.

If output_file is "-", then the output will be written to stdout.

If you have installed the script into the default location, you
can logout and login again to use the autocompletion script.

Alternatively, you can source the script directly

    . /path/to/my_bash_completion_scripts/rclone

and the autocompletion functionality will be added to your
current shell.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		out := "/etc/bash_completion.d/rclone"
		if len(args) > 0 {
			if args[0] == "-" {
				err := cmd.Root.GenBashCompletionV2(os.Stdout, false)
				if err != nil {
					fs.Fatal(nil, fmt.Sprint(err))
				}
				return
			}
			out = args[0]
		}
		err := cmd.Root.GenBashCompletionFileV2(out, false)
		if err != nil {
			fs.Fatal(nil, fmt.Sprint(err))
		}
	},
}
