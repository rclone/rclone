// Package dlnaflags provides utility functionality to DLNA.
package dlnaflags

import (
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/pflag"
)

// Help contains the text for the command line help and manual.
var Help = `### Server options

Use ` + "`--addr`" + ` to specify which IP address and port the server should
listen on, e.g. ` + "`--addr 1.2.3.4:8000` or `--addr :8080`" + ` to listen to all
IPs.

Use ` + "`--name`" + ` to choose the friendly server name, which is by
default "rclone (hostname)".

Use ` + "`--log-trace` in conjunction with `-vv`" + ` to enable additional debug
logging of all UPNP traffic.

`

// OptionsInfo descripts the Options in use
var OptionsInfo = fs.Options{{
	Name:    "addr",
	Default: ":7879",
	Help:    "The ip:port or :port to bind the DLNA http server to",
}, {
	Name:    "name",
	Default: "",
	Help:    "Name of DLNA server",
}, {
	Name:    "log_trace",
	Default: false,
	Help:    "Enable trace logging of SOAP traffic",
}, {
	Name:    "interface",
	Default: []string{},
	Help:    "The interface to use for SSDP (repeat as necessary)",
}, {
	Name:    "announce_interval",
	Default: fs.Duration(12 * time.Minute),
	Help:    "The interval between SSDP announcements",
}}

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "dlna", Opt: &Opt, Options: OptionsInfo})
}

// Options is the type for DLNA serving options.
type Options struct {
	ListenAddr       string      `config:"addr"`
	FriendlyName     string      `config:"name"`
	LogTrace         bool        `config:"log_trace"`
	InterfaceNames   []string    `config:"interface"`
	AnnounceInterval fs.Duration `config:"announce_interval"`
}

// Opt contains the options for DLNA serving.
var Opt Options

// AddFlags add the command line flags for DLNA serving.
func AddFlags(flagSet *pflag.FlagSet) {
	flags.AddFlagsFromOptions(flagSet, "", OptionsInfo)
}
