// Package changenotify tests rclone's changenotify support
package changenotify

import (
	"context"
	"errors"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/test"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/cobra"
)

var (
	pollInterval = 10 * time.Second
)

func init() {
	test.Command.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.DurationVarP(cmdFlags, &pollInterval, "poll-interval", "", pollInterval, "Time to wait between polling for changes", "")
}

var commandDefinition = &cobra.Command{
	Use:   "changenotify remote:",
	Short: `Log any change notify requests for the remote passed in.`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.56",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		ctx := context.Background()

		// Start polling function
		features := f.Features()
		if do := features.ChangeNotify; do != nil {
			pollChan := make(chan time.Duration)
			do(ctx, changeNotify, pollChan)
			pollChan <- pollInterval
			fs.Logf(nil, "Waiting for changes, polling every %v", pollInterval)
		} else {
			return errors.New("poll-interval is not supported by this remote")
		}
		select {}
	},
}

// changeNotify invalidates the directory cache for the relativePath
// passed in.
//
// if entryType is a directory it invalidates the parent of the directory too.
func changeNotify(relativePath string, entryType fs.EntryType) {
	fs.Logf(nil, "%q: %v", relativePath, entryType)
}
