// +build !plan9

package cachestats

import (
	"encoding/json"
	"fmt"

	"github.com/ncw/rclone/cache"
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "cachestats source:",
	Short: `Print cache stats for a remote`,
	Long: `
Print cache stats for a remote in JSON format
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)

		_, configName, _, err := fs.ParseRemote(args[0])
		if err != nil {
			fs.Errorf("cachestats", "%s", err.Error())
			return
		}

		if !fs.ConfigFileGetBool(configName, "read_only", false) {
			fs.ConfigFileSet(configName, "read_only", "true")
			defer fs.ConfigFileDeleteKey(configName, "read_only")
		}

		fsrc := cmd.NewFsSrc(args)
		cmd.Run(true, true, command, func() error {
			var fsCache *cache.Fs
			fsCache, ok := fsrc.(*cache.Fs)
			if !ok {
				fsCache, ok = fsrc.Features().UnWrap().(*cache.Fs)
				if !ok {
					return errors.Errorf("%s: is not a cache remote", fsrc.Name())
				}
			}
			m, err := fsCache.Stats()

			raw, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", string(raw))
			return nil
		})
	},
}
