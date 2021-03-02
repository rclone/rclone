package rcd

import (
	"context"
	"log"
	"sync"

	sysdnotify "github.com/iguanesolutions/go-systemd/v5/notify"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/rc/rcflags"
	"github.com/rclone/rclone/fs/rc/rcserver"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "rcd <path to files to serve>*",
	Short: `Run rclone listening to remote control commands only.`,
	Long: `
This runs rclone so that it only listens to remote control commands.

This is useful if you are controlling rclone via the rc API.

If you pass in a path to a directory, rclone will serve that directory
for GET requests on the URL passed in.  It will also open the URL in
the browser when rclone is run.

See the [rc documentation](/rc/) for more info on the rc flags.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		if rcflags.Opt.Enabled {
			log.Fatalf("Don't supply --rc flag when using rcd")
		}

		// Start the rc
		rcflags.Opt.Enabled = true
		if len(args) > 0 {
			rcflags.Opt.Files = args[0]
		}

		s, err := rcserver.Start(context.Background(), &rcflags.Opt)
		if err != nil {
			log.Fatalf("Failed to start remote control: %v", err)
		}
		if s == nil {
			log.Fatal("rc server not configured")
		}

		// Notify stopping on exit
		var finaliseOnce sync.Once
		finalise := func() {
			finaliseOnce.Do(func() {
				_ = sysdnotify.Stopping()
			})
		}
		fnHandle := atexit.Register(finalise)
		defer atexit.Unregister(fnHandle)

		// Notify ready to systemd
		if err := sysdnotify.Ready(); err != nil {
			log.Fatalf("failed to notify ready to systemd: %v", err)
		}

		s.Wait()
		finalise()
	},
}
