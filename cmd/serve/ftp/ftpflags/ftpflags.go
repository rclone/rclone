package ftpflags

import (
	"github.com/ncw/rclone/cmd/serve/ftp/ftpopt"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/rc"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = ftpopt.DefaultOpt
)

// AddFlagsPrefix adds flags for the ftpopt
func AddFlagsPrefix(flagSet *pflag.FlagSet, prefix string, Opt *ftpopt.Options) {
	rc.AddOption("ftp", &Opt)
	flags.StringVarP(flagSet, &Opt.ListenAddr, prefix+"addr", "", Opt.ListenAddr, "IPaddress:Port or :Port to bind server to.")
	flags.StringVarP(flagSet, &Opt.PassivePorts, prefix+"passive-port", "", Opt.PassivePorts, "Passive port range to use.")
	flags.StringVarP(flagSet, &Opt.BasicUser, prefix+"user", "", Opt.BasicUser, "User name for authentication.")
	flags.StringVarP(flagSet, &Opt.BasicPass, prefix+"pass", "", Opt.BasicPass, "Password for authentication. (empty value allow every password)")
}

// AddFlags adds flags for the httplib
func AddFlags(flagSet *pflag.FlagSet) {
	AddFlagsPrefix(flagSet, "", &Opt)
}
