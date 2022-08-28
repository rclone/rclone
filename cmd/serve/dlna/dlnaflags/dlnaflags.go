// Package dlnaflags provides utility functionality to DLNA.
package dlnaflags

import (
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/spf13/pflag"
)

// Help contains the text for the command line help and manual.
var Help = `
### Server options

Use ` + "`--addr`" + ` to specify which IP address and port the server should
listen on, e.g. ` + "`--addr 1.2.3.4:8000` or `--addr :8080`" + ` to listen to all
IPs.

Use ` + "`--name`" + ` to choose the friendly server name, which is by
default "rclone (hostname)".

Use ` + "`--log-trace` in conjunction with `-vv`" + ` to enable additional debug
logging of all UPNP traffic.
`

// Options is the type for DLNA serving options.
type Options struct {
	ListenAddr     string
	FriendlyName   string
	LogTrace       bool
	InterfaceNames []string
}

// DefaultOpt contains the defaults options for DLNA serving.
var DefaultOpt = Options{
	ListenAddr:     ":7879",
	FriendlyName:   "",
	LogTrace:       false,
	InterfaceNames: []string{},
}

// Opt contains the options for DLNA serving.
var (
	Opt = DefaultOpt
)

func addFlagsPrefix(flagSet *pflag.FlagSet, prefix string, Opt *Options) {
	rc.AddOption("dlna", &Opt)
	flags.StringVarP(flagSet, &Opt.ListenAddr, prefix+"addr", "", Opt.ListenAddr, "The ip:port or :port to bind the DLNA http server to")
	flags.StringVarP(flagSet, &Opt.FriendlyName, prefix+"name", "", Opt.FriendlyName, "Name of DLNA server")
	flags.BoolVarP(flagSet, &Opt.LogTrace, prefix+"log-trace", "", Opt.LogTrace, "Enable trace logging of SOAP traffic")
	flags.StringArrayVarP(flagSet, &Opt.InterfaceNames, prefix+"interface", "", Opt.InterfaceNames, "The interface to use for SSDP (repeat as necessary)")
}

// AddFlags add the command line flags for DLNA serving.
func AddFlags(flagSet *pflag.FlagSet) {
	addFlagsPrefix(flagSet, "", &Opt)
}
