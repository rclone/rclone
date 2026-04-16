//go:build !windows && !plan9 && !(linux && (386 || arm || mips || mipsle))

// Package smb implements an SMB server to serve an rclone VFS
package smb

import (
	"context"
	"fmt"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/systemd"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// OptionsInfo describes the Options in use
var OptionsInfo = fs.Options{{
	Name:    "addr",
	Default: "localhost:445",
	Help:    "IPaddress:Port or :Port to bind server to",
}, {
	Name:    "user",
	Default: "",
	Help:    "User name for authentication",
}, {
	Name:    "pass",
	Default: "",
	Help:    "Password for authentication",
}, {
	Name:    "no_auth",
	Default: false,
	Help:    "Allow connections with no authentication if set",
}, {
	Name:    "share_name",
	Default: "rclone",
	Help:    "Name of the SMB share",
}}

// Options contains options for the SMB server
type Options struct {
	ListenAddr string `config:"addr"`       // Port to listen on
	User       string `config:"user"`       // single username
	Pass       string `config:"pass"`       // password for user
	NoAuth     bool   `config:"no_auth"`    // allow no authentication on connections
	ShareName  string `config:"share_name"` // name of the SMB share
}

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "smb-serve", Opt: &Opt, Options: OptionsInfo})
}

// Opt is options set by command line flags
var Opt Options

// AddFlags adds flags for the SMB server
func AddFlags(flagSet *pflag.FlagSet, Opt *Options) {
	flags.AddFlagsFromOptions(flagSet, "", OptionsInfo)
}

func init() {
	vfsflags.AddFlags(Command.Flags())
	proxyflags.AddFlags(Command.Flags())
	AddFlags(Command.Flags(), &Opt)
	serve.Command.AddCommand(Command)
	serve.AddRc("smb", func(ctx context.Context, f fs.Fs, in rc.Params) (serve.Handle, error) {
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

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "smb remote:path",
	Short: `Serve the remote over SMB.`,
	Long: `Run an SMB server to serve a remote over SMB. This can be used
with an SMB client or you can make a remote of type [smb](/smb) to use with it.

You can use the [filter](/filtering) flags (e.g. ` + "`--include`, `--exclude`" + `)
to control what is served.

The server will log errors.  Use ` + "`-v`" + ` to see access logs.

` + "`--bwlimit`" + ` will be respected for file transfers.
Use ` + "`--stats`" + ` to control the stats printing.

You must provide some means of authentication, either with
` + "`--user`/`--pass`" + `, an ` + "`--auth-proxy`" + `, or set the ` + "`--no-auth`" + ` flag for no
authentication when logging in.

By default the server binds to localhost:445 - if you want it to be
reachable externally then supply ` + "`--addr :445`" + ` for example.

Note that port 445 typically requires root/administrator privileges.
Use ` + "`--addr :1445`" + ` or similar to use a non-privileged port.

The remote will be served as a single SMB share. The share name
defaults to "rclone" and can be changed with ` + "`--share-name`" + `.

Note that the default of ` + "`--vfs-cache-mode off`" + ` is fine for the rclone
smb backend, but it may not be with other SMB clients.

This command uses the library https://github.com/macos-fuse-t/go-smb2
which the author has given special consent for rclone to build and
distribute under the rclone licensing terms.

` + strings.TrimSpace(vfs.Help()+proxy.Help),
	Annotations: map[string]string{
		"versionIntroduced": "v1.74",
		"groups":            "Filter",
	},
	Run: func(command *cobra.Command, args []string) {
		var f fs.Fs
		if proxy.Opt.AuthProxy == "" {
			cmd.CheckArgs(1, 1, command, args)
			f = cmd.NewFsSrc(args)
		} else {
			cmd.CheckArgs(0, 0, command, args)
		}
		cmd.Run(false, true, command, func() error {
			s, err := newServer(context.Background(), f, &Opt, &vfscommon.Opt, &proxy.Opt)
			if err != nil {
				fs.Fatal(nil, fmt.Sprint(err))
			}
			defer systemd.Notify()()
			return s.Serve()
		})
	},
}
