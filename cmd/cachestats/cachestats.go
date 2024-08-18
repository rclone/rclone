//go:build !plan9 && !js

// Package cachestats provides the cachestats command.
package cachestats

import (
	"encoding/json"
	"fmt"

	"github.com/rclone/rclone/backend/cache"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "cachestats source:",
	Short: `Print cache stats for a remote`,
	Long: `Print cache stats for a remote in JSON format
`,
	Hidden: true,
	Annotations: map[string]string{
		"versionIntroduced": "v1.39",
		"status":            "Deprecated",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fs.Logf(nil, `"rclone cachestats" is deprecated, use "rclone backend stats %s" instead`, args[0])

		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			var fsCache *cache.Fs
			fsCache, ok := fsrc.(*cache.Fs)
			if !ok {
				unwrap := fsrc.Features().UnWrap
				if unwrap != nil {
					fsCache, ok = unwrap().(*cache.Fs)
				}
				if !ok {
					return fmt.Errorf("%s: is not a cache remote", fsrc.Name())
				}
			}
			m, err := fsCache.Stats()
			if err != nil {
				return err
			}

			raw, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", string(raw))
			return nil
		})
	},
}
