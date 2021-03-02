package size

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var jsonOutput bool

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &jsonOutput, "json", "", false, "format output as JSON")
}

var commandDefinition = &cobra.Command{
	Use:   "size remote:path",
	Short: `Prints the total size and number of objects in remote:path.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			var err error
			var results struct {
				Count int64 `json:"count"`
				Bytes int64 `json:"bytes"`
			}

			results.Count, results.Bytes, err = operations.Count(context.Background(), fsrc)
			if err != nil {
				return err
			}

			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(results)
			}

			fmt.Printf("Total objects: %d\n", results.Count)
			fmt.Printf("Total size: %s (%d bytes)\n", fs.SizeSuffix(results.Bytes).ByteUnit(), results.Bytes)

			return nil
		})
	},
}
