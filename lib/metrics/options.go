package metrics

import (
	"github.com/rclone/rclone/fs"
	configflags "github.com/rclone/rclone/fs/config/flags"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/spf13/pflag"
)

// Options holds the configuration for the metrics server
type Options struct {
	HTTP     libhttp.Config         `config:"metrics"`
	Auth     libhttp.AuthConfig     `config:"metrics"`
	Template libhttp.TemplateConfig `config:"metrics"`
}

var (
	opt     Options
	rcEmbed bool

	optionsInfo = fs.Options{{
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
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "metrics", Opt: &opt, Options: optionsInfo})
	configflags.AddFlagsFromOptions(pflag.CommandLine, "", optionsInfo)
}

// Enabled returns whether the metrics server is enabled
func Enabled() bool {
	return len(opt.HTTP.ListenAddr) > 0 || rcEmbed
}

// RCEmbed returns whether the metrics server is embedded in the rc server
func RCEmbed() bool {
	return rcEmbed
}

// SetRCEmbed sets whether the metrics server is embedded in the rc server
func SetRCEmbed(enabled bool) {
	rcEmbed = enabled
}
