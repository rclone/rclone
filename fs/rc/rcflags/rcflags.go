// Package rcflags implements command line flags to set up the remote control
package rcflags

import (
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/spf13/pflag"
)

// FlagPrefix is the prefix used to uniquely identify command line flags.
const FlagPrefix = "rc-"

// Options set by command line flags
var (
	Opt = rc.DefaultOpt
)

// AddFlags adds the remote control flags to the flagSet
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOption("rc", &Opt)
	flags.BoolVarP(flagSet, &Opt.Enabled, "rc", "", false, "Enable the remote control server", "RC")
	flags.StringVarP(flagSet, &Opt.Files, "rc-files", "", "", "Path to local files to serve on the HTTP server", "RC")
	flags.BoolVarP(flagSet, &Opt.Serve, "rc-serve", "", false, "Enable the serving of remote objects", "RC")
	flags.BoolVarP(flagSet, &Opt.ServeNoModTime, "rc-serve-no-modtime", "", false, "Don't read the modification time (can speed things up)", "RC")
	flags.BoolVarP(flagSet, &Opt.NoAuth, "rc-no-auth", "", false, "Don't require auth for certain methods", "RC")
	flags.BoolVarP(flagSet, &Opt.WebUI, "rc-web-gui", "", false, "Launch WebGUI on localhost", "RC")
	flags.BoolVarP(flagSet, &Opt.WebGUIUpdate, "rc-web-gui-update", "", false, "Check and update to latest version of web gui", "RC")
	flags.BoolVarP(flagSet, &Opt.WebGUIForceUpdate, "rc-web-gui-force-update", "", false, "Force update to latest version of web gui", "RC")
	flags.BoolVarP(flagSet, &Opt.WebGUINoOpenBrowser, "rc-web-gui-no-open-browser", "", false, "Don't open the browser automatically", "RC")
	flags.StringVarP(flagSet, &Opt.WebGUIFetchURL, "rc-web-fetch-url", "", "https://api.github.com/repos/rclone/rclone-webui-react/releases/latest", "URL to fetch the releases for webgui", "RC")
	flags.BoolVarP(flagSet, &Opt.EnableMetrics, "rc-enable-metrics", "", false, "Enable prometheus metrics on /metrics", "RC")
	flags.DurationVarP(flagSet, &Opt.JobExpireDuration, "rc-job-expire-duration", "", Opt.JobExpireDuration, "Expire finished async jobs older than this value", "RC")
	flags.DurationVarP(flagSet, &Opt.JobExpireInterval, "rc-job-expire-interval", "", Opt.JobExpireInterval, "Interval to check for expired async jobs", "RC")
	Opt.HTTP.AddFlagsPrefix(flagSet, FlagPrefix)
	Opt.Auth.AddFlagsPrefix(flagSet, FlagPrefix)
	Opt.Template.AddFlagsPrefix(flagSet, FlagPrefix)
}
