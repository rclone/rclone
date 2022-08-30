//go:build !plan9
// +build !plan9

// Package sftp implements an SFTP server to serve an rclone VFS
package sftp

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Options contains options for the http Server
type Options struct {
	ListenAddr     string   // Port to listen on
	HostKeys       []string // Paths to private host keys
	AuthorizedKeys string   // Path to authorized keys file
	User           string   // single username
	Pass           string   // password for user
	NoAuth         bool     // allow no authentication on connections
	Stdio          bool     // serve on stdio
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	ListenAddr:     "localhost:2022",
	AuthorizedKeys: "~/.ssh/authorized_keys",
}

// Opt is options set by command line flags
var Opt = DefaultOpt

// AddFlags adds flags for the sftp
func AddFlags(flagSet *pflag.FlagSet, Opt *Options) {
	rc.AddOption("sftp", &Opt)
	flags.StringVarP(flagSet, &Opt.ListenAddr, "addr", "", Opt.ListenAddr, "IPaddress:Port or :Port to bind server to")
	flags.StringArrayVarP(flagSet, &Opt.HostKeys, "key", "", Opt.HostKeys, "SSH private host key file (Can be multi-valued, leave blank to auto generate)")
	flags.StringVarP(flagSet, &Opt.AuthorizedKeys, "authorized-keys", "", Opt.AuthorizedKeys, "Authorized keys file")
	flags.StringVarP(flagSet, &Opt.User, "user", "", Opt.User, "User name for authentication")
	flags.StringVarP(flagSet, &Opt.Pass, "pass", "", Opt.Pass, "Password for authentication")
	flags.BoolVarP(flagSet, &Opt.NoAuth, "no-auth", "", Opt.NoAuth, "Allow connections with no authentication if set")
	flags.BoolVarP(flagSet, &Opt.Stdio, "stdio", "", Opt.Stdio, "Run an sftp server on stdin/stdout")
}

func init() {
	vfsflags.AddFlags(Command.Flags())
	proxyflags.AddFlags(Command.Flags())
	AddFlags(Command.Flags(), &Opt)
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "sftp remote:path",
	Short: `Serve the remote over SFTP.`,
	Long: `Run an SFTP server to serve a remote over SFTP. This can be used
with an SFTP client or you can make a remote of type [sftp](/sftp) to use with it.

You can use the [filter](/filtering) flags (e.g. ` + "`--include`, `--exclude`" + `)
to control what is served.

The server will respond to a small number of shell commands, mainly
md5sum, sha1sum and df, which enable it to provide support for checksums
and the about feature when accessed from an sftp remote.

Note that this server uses standard 32 KiB packet payload size, which
means you must not configure the client to expect anything else, e.g.
with the [chunk_size](/sftp/#sftp-chunk-size) option on an sftp remote.

The server will log errors.  Use ` + "`-v`" + ` to see access logs.

` + "`--bwlimit`" + ` will be respected for file transfers.
Use ` + "`--stats`" + ` to control the stats printing.

You must provide some means of authentication, either with
` + "`--user`/`--pass`" + `, an authorized keys file (specify location with
` + "`--authorized-keys`" + ` - the default is the same as ssh), an
` + "`--auth-proxy`" + `, or set the ` + "`--no-auth`" + ` flag for no
authentication when logging in.

If you don't supply a host ` + "`--key`" + ` then rclone will generate rsa, ecdsa
and ed25519 variants, and cache them for later use in rclone's cache
directory (see ` + "`rclone help flags cache-dir`" + `) in the "serve-sftp"
directory.

By default the server binds to localhost:2022 - if you want it to be
reachable externally then supply ` + "`--addr :2022`" + ` for example.

Note that the default of ` + "`--vfs-cache-mode off`" + ` is fine for the rclone
sftp backend, but it may not be with other SFTP clients.

If ` + "`--stdio`" + ` is specified, rclone will serve SFTP over stdio, which can
be used with sshd via ~/.ssh/authorized_keys, for example:

    restrict,command="rclone serve sftp --stdio ./photos" ssh-rsa ...

On the client you need to set ` + "`--transfers 1`" + ` when using ` + "`--stdio`" + `.
Otherwise multiple instances of the rclone server are started by OpenSSH
which can lead to "corrupted on transfer" errors. This is the case because
the client chooses indiscriminately which server to send commands to while
the servers all have different views of the state of the filing system.

The "restrict" in authorized_keys prevents SHA1SUMs and MD5SUMs from beeing
used. Omitting "restrict" and using  ` + "`--sftp-path-override`" + ` to enable
checksumming is possible but less secure and you could use the SFTP server
provided by OpenSSH in this case.

` + vfs.Help + proxy.Help,
	Run: func(command *cobra.Command, args []string) {
		var f fs.Fs
		if proxyflags.Opt.AuthProxy == "" {
			cmd.CheckArgs(1, 1, command, args)
			f = cmd.NewFsSrc(args)
		} else {
			cmd.CheckArgs(0, 0, command, args)
		}
		cmd.Run(false, true, command, func() error {
			if Opt.Stdio {
				return serveStdio(f)
			}
			s := newServer(context.Background(), f, &Opt)
			err := s.Serve()
			if err != nil {
				return err
			}
			s.Wait()
			return nil
		})
	},
}
