package size

import (
	"fmt"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
}

var commandDefintion = &cobra.Command{
	Use:   "size remote:path",
	Short: `Prints the total size and number of objects in remote:path.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			objects, size, err := fs.Count(fsrc)
			if err != nil {
				return err
			}
			fmt.Printf("Total objects: %d\n", objects)
			fmt.Printf("Total size: %s (%d Bytes)\n", fs.SizeSuffix(size).Unit("Bytes"), size)
			return nil
		})
	},
}
