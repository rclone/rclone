// Package rcflags implements command line flags to set up the remote control
package rcflags

import (
	"github.com/ncw/rclone/cmd/serve/httplib/httpflags"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/rc"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = rc.DefaultOpt
)

// AddFlags adds the remote control flags to the flagSet
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOption("rc", &Opt)
	flags.BoolVarP(flagSet, &Opt.Enabled, "rc", "", false, "Enable the remote control server.")
	flags.StringVarP(flagSet, &Opt.Files, "rc-files", "", "", "Path to local files to serve on the HTTP server.")
	flags.BoolVarP(flagSet, &Opt.Serve, "rc-serve", "", false, "Enable the serving of remote objects.")
	flags.BoolVarP(flagSet, &Opt.NoAuth, "rc-no-auth", "", false, "Don't require auth for certain methods.")
	httpflags.AddFlagsPrefix(flagSet, "rc-", &Opt.HTTPOptions)
}
