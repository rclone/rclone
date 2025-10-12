package s3

import (
	"context"
	_ "embed"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	httplib "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
)

// OptionsInfo describes the Options in use
var OptionsInfo = fs.Options{{
	Name:    "force_path_style",
	Default: true,
	Help:    "If true use path style access if false use virtual hosted style",
}, {
	Name:    "etag_hash",
	Default: "MD5",
	Help:    "Which hash to use for the ETag, or auto or blank for off",
}, {
	Name:    "auth_key",
	Default: []string{},
	Help:    "Set key pair for v4 authorization: access_key_id,secret_access_key",
}, {
	Name:    "no_cleanup",
	Default: false,
	Help:    "Not to cleanup empty folder after object is deleted",
}}.
	Add(httplib.ConfigInfo).
	Add(httplib.AuthConfigInfo)

// Options contains options for the s3 Server
type Options struct {
	//TODO add more options
	ForcePathStyle bool     `config:"force_path_style"`
	EtagHash       string   `config:"etag_hash"`
	AuthKey        []string `config:"auth_key"`
	NoCleanup      bool     `config:"no_cleanup"`
	Auth           httplib.AuthConfig
	HTTP           httplib.Config
}

// Opt is options set by command line flags
var Opt Options

const flagPrefix = ""

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "s3", Opt: &Opt, Options: OptionsInfo})
	flagSet := Command.Flags()
	flags.AddFlagsFromOptions(flagSet, "", OptionsInfo)
	vfsflags.AddFlags(flagSet)
	proxyflags.AddFlags(flagSet)
	serve.Command.AddCommand(Command)
	serve.AddRc("s3", func(ctx context.Context, f fs.Fs, in rc.Params) (serve.Handle, error) {
		// Read VFS Opts
		var vfsOpt = vfscommon.Opt // set default opts
		err := configstruct.SetAny(in, &vfsOpt)
		if err != nil {
			return nil, err
		}
		// Read Proxy Opts
		var proxyOpt = proxy.Opt // set default opts
		err = configstruct.SetAny(in, &proxyOpt)
		if err != nil {
			return nil, err
		}
		// Read opts
		var opt = Opt // set default opts
		err = configstruct.SetAny(in, &opt)
		if err != nil {
			return nil, err
		}
		// Create server
		return newServer(ctx, f, &opt, &vfsOpt, &proxyOpt)
	})
}

//go:embed serve_s3.md
var serveS3Help string

// help returns the help string cleaned up to simplify appending
func help() string {
	return strings.TrimSpace(serveS3Help) + "\n\n"
}

// Command definition for cobra
var Command = &cobra.Command{
	Annotations: map[string]string{
		"versionIntroduced": "v1.65",
		"groups":            "Filter",
		"status":            "Experimental",
	},
	Use:   "s3 remote:path",
	Short: `Serve remote:path over s3.`,
	Long:  help() + strings.TrimSpace(httplib.AuthHelp(flagPrefix)+httplib.Help(flagPrefix)+vfs.Help()),
	RunE: func(command *cobra.Command, args []string) error {
		var f fs.Fs
		if proxy.Opt.AuthProxy == "" {
			cmd.CheckArgs(1, 1, command, args)
			f = cmd.NewFsSrc(args)
		} else {
			cmd.CheckArgs(0, 0, command, args)
		}

		cmd.Run(false, false, command, func() error {
			s, err := newServer(context.Background(), f, &Opt, &vfscommon.Opt, &proxy.Opt)
			if err != nil {
				return err
			}
			return s.Serve()
		})
		return nil
	},
}
