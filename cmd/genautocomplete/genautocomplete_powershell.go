package genautocomplete

import (
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	completionDefinition.AddCommand(powershellCommandDefinition)
}

var powershellCommandDefinition = &cobra.Command{
	Use:   "powershell [output_file]",
	Short: `Output powershell completion script for rclone.`,
	Long: `Generate the autocompletion script for powershell.

To load completions in your current shell session:

    rclone completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.

If output_file is "-" or missing, then the output will be written to stdout.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		if len(args) == 0 || (len(args) > 0 && args[0] == "-") {
			err := cmd.Root.GenPowerShellCompletion(os.Stdout)
			if err != nil {
				fs.Fatal(nil, fmt.Sprint(err))
			}
			return
		}
		err := cmd.Root.GenPowerShellCompletionFile(args[0])
		if err != nil {
			fs.Fatal(nil, fmt.Sprint(err))
		}
	},
}
