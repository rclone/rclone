// Package httpflags provides utility functionality to HTTP.
package httpflags

import (
	"github.com/rclone/rclone/cmd/serve/httplib"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = httplib.DefaultOpt
)

// AddFlagsPrefix adds flags for the httplib
func AddFlagsPrefix(flagSet *pflag.FlagSet, prefix string, Opt *httplib.Options) {
	rc.AddOption(prefix+"http", &Opt)
	flags.StringVarP(flagSet, &Opt.ListenAddr, prefix+"addr", "", Opt.ListenAddr, "IPaddress:Port or :Port to bind server to")
	flags.DurationVarP(flagSet, &Opt.ServerReadTimeout, prefix+"server-read-timeout", "", Opt.ServerReadTimeout, "Timeout for server reading data")
	flags.DurationVarP(flagSet, &Opt.ServerWriteTimeout, prefix+"server-write-timeout", "", Opt.ServerWriteTimeout, "Timeout for server writing data")
	flags.IntVarP(flagSet, &Opt.MaxHeaderBytes, prefix+"max-header-bytes", "", Opt.MaxHeaderBytes, "Maximum size of request header")
	flags.StringVarP(flagSet, &Opt.SslCert, prefix+"cert", "", Opt.SslCert, "SSL PEM key (concatenation of certificate and CA certificate)")
	flags.StringVarP(flagSet, &Opt.SslKey, prefix+"key", "", Opt.SslKey, "SSL PEM Private key")
	flags.StringVarP(flagSet, &Opt.ClientCA, prefix+"client-ca", "", Opt.ClientCA, "Client certificate authority to verify clients with")
	flags.StringVarP(flagSet, &Opt.HtPasswd, prefix+"htpasswd", "", Opt.HtPasswd, "htpasswd file - if not provided no authentication is done")
	flags.StringVarP(flagSet, &Opt.Realm, prefix+"realm", "", Opt.Realm, "Realm for authentication")
	flags.StringVarP(flagSet, &Opt.BasicUser, prefix+"user", "", Opt.BasicUser, "User name for authentication")
	flags.StringVarP(flagSet, &Opt.BasicPass, prefix+"pass", "", Opt.BasicPass, "Password for authentication")
	flags.StringVarP(flagSet, &Opt.BaseURL, prefix+"baseurl", "", Opt.BaseURL, "Prefix for URLs - leave blank for root")
	flags.StringVarP(flagSet, &Opt.Template, prefix+"template", "", Opt.Template, "User-specified template")

}

// AddFlags adds flags for the httplib
func AddFlags(flagSet *pflag.FlagSet) {
	AddFlagsPrefix(flagSet, "", &Opt)
}
