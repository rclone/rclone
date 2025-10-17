package metrics

import (
	"github.com/rclone/rclone/fs"
	configflags "github.com/rclone/rclone/fs/config/flags"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/spf13/pflag"
)

type Options struct {
	HTTP     libhttp.Config         `config:"metrics"`
	Auth     libhttp.AuthConfig     `config:"metrics"`
	Template libhttp.TemplateConfig `config:"metrics"`
}

var (
	Opt     Options
	rcEmbed bool

	OptionsInfo = fs.Options{{
		Name:    "metrics_addr",
		Default: []string{},
		Help:    "IPaddress:Port or :Port to bind metrics server to",
		Groups:  "Metrics",
	}}.
		AddPrefix(libhttp.ConfigInfo, "metrics", "Metrics").
		AddPrefix(libhttp.AuthConfigInfo, "metrics", "Metrics").
		AddPrefix(libhttp.TemplateConfigInfo, "metrics", "Metrics")
)

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "metrics", Opt: &Opt, Options: OptionsInfo})
	configflags.AddFlagsFromOptions(pflag.CommandLine, "", OptionsInfo)
}

func Enabled() bool {
	return len(Opt.HTTP.ListenAddr) > 0 || rcEmbed
}

func RCEmbed() bool {
	return rcEmbed
}

func SetRCEmbed(enabled bool) {
	rcEmbed = enabled
}
