// Package proxyflags implements command line flags to set up a proxy
package proxyflags

import (
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = proxy.DefaultOpt
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	flags.StringVarP(flagSet, &Opt.AuthProxy, "auth-proxy", "", Opt.AuthProxy, "A program to use to create the backend from the auth", "")
}
