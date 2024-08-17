// Package rcd provides the rcd command.
package rcd

import (
	"context"
	"log"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/rcflags"
	"github.com/rclone/rclone/fs/rc/rcserver"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/systemd"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "rcd <path to files to serve>*",
	Short: `Run rclone listening to remote control commands only.`,
	Long: `This runs rclone so that it only listens to remote control commands.

This is useful if you are controlling rclone via the rc API.

If you pass in a path to a directory, rclone will serve that directory
for GET requests on the URL passed in.  It will also open the URL in
the browser when rclone is run.

See the [rc documentation](/rc/) for more info on the rc flags.

` + libhttp.Help(rcflags.FlagPrefix) + libhttp.TemplateHelp(rcflags.FlagPrefix) + libhttp.AuthHelp(rcflags.FlagPrefix),
	Annotations: map[string]string{
		"versionIntroduced": "v1.45",
		"groups":            "RC",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1, command, args)
		if rc.Opt.Enabled {
			log.Fatalf("Don't supply --rc flag when using rcd")
		}

		// Start the rc
		rc.Opt.Enabled = true
		if len(args) > 0 {
			rc.Opt.Files = args[0]
		}

		ctx := context.Background()

		s, err := rcserver.Start(ctx, &rc.Opt)
		if err != nil {
			log.Fatalf("Failed to start remote control: %v", err)
		}
		if s == nil {
			log.Fatal("rc server not configured")
		}

		if rc.Opt.MetricsEnabled && !rcserver.SlicesEqual(rc.Opt.MetricsHTTP.ListenAddr, rc.Opt.HTTP.ListenAddr) {
			go func() {
				_, err := rcserver.MetricsStart(ctx, &rc.Opt)
				if err != nil {
					log.Fatalf("Failed to start metrics server: %v", err)
				}
			}()
		}

		// Notify stopping on exit
		defer systemd.Notify()()

		s.Wait()
	},
}
