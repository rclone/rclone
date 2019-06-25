package whirlpoolsum

import (
	"context"
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "whirlpoolsum remote:path",
	Short: `Produces an sha1sum file for all the objects in the path.`,
	Long: `
Produces an whirlpoolsum file for all the objects in the path.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return operations.WhirlpoolHashSum(context.Background(), fsrc, os.Stdout)
		})
	},
}
