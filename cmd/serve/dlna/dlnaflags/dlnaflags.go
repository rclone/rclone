package dlnaflags

import (
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/rc"
	"github.com/spf13/pflag"
)

// Help contains the text for the command line help and manual.
var Help = `
### Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.

`

// Options is the type for DLNA serving options.
type Options struct {
	ListenAddr string
}

// DefaultOpt contains the defaults options for DLNA serving.
var DefaultOpt = Options{
	ListenAddr: ":7879",
}

// Opt contains the options for DLNA serving.
var (
	Opt = DefaultOpt
)

func addFlagsPrefix(flagSet *pflag.FlagSet, prefix string, Opt *Options) {
	rc.AddOption("dlna", &Opt)
	flags.StringVarP(flagSet, &Opt.ListenAddr, prefix+"addr", "", Opt.ListenAddr, "ip:port or :port to bind the DLNA http server to.")
}

// AddFlags add the command line flags for DLNA serving.
func AddFlags(flagSet *pflag.FlagSet) {
	addFlagsPrefix(flagSet, "", &Opt)
}
