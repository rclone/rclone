// Package histogram provides the histogram test command.
package histogram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/test"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/walk"
	"github.com/spf13/cobra"
)

func init() {
	test.Command.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "histogram [remote:path]",
	Short: `Makes a histogram of file name characters.`,
	Long: `This command outputs JSON which shows the histogram of characters used
in filenames in the remote:path specified.

The data doesn't contain any identifying information but is useful for
the rclone developers when developing filename compression.`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.55",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsDir(args)
		ctx := context.Background()
		ci := fs.GetConfig(ctx)
		cmd.Run(false, false, command, func() error {
			var hist [256]int64
			err := walk.ListR(ctx, f, "", false, ci.MaxDepth, walk.ListObjects, func(entries fs.DirEntries) error {
				for _, entry := range entries {
					base := path.Base(entry.Remote())
					for i := range base {
						hist[base[i]]++
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			// enc.SetIndent("", "\t")
			err = enc.Encode(&hist)
			if err != nil {
				return err
			}
			fmt.Println()
			return nil
		})
	},
}
