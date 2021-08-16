// Package logflags implements command line flags to set up the log
package logflags

import (
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/rc"
	"github.com/spf13/pflag"
)

// AddFlags adds the log flags to the flagSet
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOption("log", &log.Opt)

	flags.StringVarP(flagSet, &log.Opt.File, "log-file", "", log.Opt.File, "Log everything to this file")
	flags.StringVarP(flagSet, &log.Opt.Format, "log-format", "", log.Opt.Format, "Comma separated list of log format options")
	flags.BoolVarP(flagSet, &log.Opt.UseSyslog, "syslog", "", log.Opt.UseSyslog, "Use Syslog for logging")
	flags.StringVarP(flagSet, &log.Opt.SyslogFacility, "syslog-facility", "", log.Opt.SyslogFacility, "Facility for syslog, e.g. KERN,USER,...")
	flags.BoolVarP(flagSet, &log.Opt.LogSystemdSupport, "log-systemd", "", log.Opt.LogSystemdSupport, "Activate systemd integration for the logger")
}
