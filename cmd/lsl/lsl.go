// Package lsl provides the lsl command.
package lsl

import (
	"context"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/ls/lshelp"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "lsl remote:path",
	Short: `List the objects in path with modification time, size and path.`,
	Long: `Lists the objects in the source path to standard output in a human
readable format with modification time, size and path. Recurses by default.

Eg

    $ rclone lsl swift:bucket
        60295 2016-06-25 18:55:41.062626927 bevajer5jef
        90613 2016-06-25 18:55:43.302607074 canole
        94467 2016-06-25 18:55:43.046609333 diwogej7
        37600 2016-06-25 18:55:40.814629136 fubuwic

` + lshelp.Help,
	Annotations: map[string]string{
		"versionIntroduced": "v1.02",
		"groups":            "Filter,Listing",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return operations.ListLong(context.Background(), fsrc, os.Stdout)
		})
	},
}
