// Package smb implements an SMB server to serve a VFS remote over the network.
package smb

import (
	"context"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// OptionsInfo describes the Options in use
var OptionsInfo = fs.Options{{
	Name:    "addr",
	Default: "127.0.0.1:1445",
	Help:    "IPaddress:Port or :Port to bind server to",
}, {
	Name:    "name",
	Default: "rclone",
	Help:    "Name of the SMB share to expose",
}, {
	Name:    "user",
	Default: "",
	Help:    "User name for authentication (empty means allow guest/no authentication)",
}, {
	Name:    "pass",
	Default: "",
	Help:    "Password for authentication",
}}

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "smb", Opt: &Opt, Options: OptionsInfo})
}

// Options contains options for the SMB Server
type Options struct {
	ListenAddr string `config:"addr"` // Address to listen on
	ShareName  string `config:"name"` // Name of the share to expose
	User       string `config:"user"` // User name for authentication
	Pass       string `config:"pass"` // Password for authentication
}

// Opt is the default set of serve smb options
var Opt Options

// AddFlags adds flags for serve smb
func AddFlags(flagSet *pflag.FlagSet) {
	flags.AddFlagsFromOptions(flagSet, "", OptionsInfo)
}

func init() {
	vfsflags.AddFlags(Command.Flags())
	AddFlags(Command.Flags())
	serve.Command.AddCommand(Command)
	serve.AddRc("smb", func(ctx context.Context, f fs.Fs, in rc.Params) (serve.Handle, error) {
		// Read VFS opts
		var vfsOpt = vfscommon.Opt
		if err := configstruct.SetAny(in, &vfsOpt); err != nil {
			return nil, err
		}
		// Read opts
		var opt = Opt
		if err := configstruct.SetAny(in, &opt); err != nil {
			return nil, err
		}
		s, err := newServer(ctx, f, &opt, &vfsOpt)
		if err != nil {
			return nil, err
		}
		return s, nil
	})
}

// Run the command
func Run(command *cobra.Command, args []string) {
	cmd.CheckArgs(1, 1, command, args)
	f := cmd.NewFsSrc(args)
	cmd.Run(false, true, command, func() error {
		s, err := newServer(context.Background(), f, &Opt, &vfscommon.Opt)
		if err != nil {
			return err
		}
		return s.Serve()
	})
}

// Command is the definition of the command
var Command = &cobra.Command{
	Use:   "smb remote:path",
	Short: `Serve the remote over SMB.`,
	Long: strings.ReplaceAll(`Run an SMB server to serve a remote over the SMB protocol.

This implements an SMB/CIFS server to serve any rclone remote over the
network. The share can be browsed and (with VFS caching) written by SMB
clients on Linux, macOS and Windows.

SMB dialects 2.0.2 through 3.0.2 are negotiated. Connections may be made
as a guest (no authentication) or with NTLM username/password
authentication, and authenticated sessions support SMB message signing.

### Server address and port

Use the |--addr| flag to set the IP address and port to listen on, e.g.
|--addr 0.0.0.0:1445| to listen on all interfaces. By default the server
listens on |127.0.0.1:1445|, the loopback interface only, so it is
reachable only from the local machine.

The default port is 1445 rather than the standard SMB port 445 so that
the server can run without administrator/root privileges and without
clashing with a system SMB service. Use |--addr <address>:445| for the
standard port.

### Authentication

By default the server runs with no authentication and grants guest
access to any client that can reach it. For this reason the default
listening address is the loopback interface. To safely expose the server
to other machines, set a username and password with |--user| and
|--pass|, or restrict access with a firewall or tunnel.

Windows SMB clients reject guest (unauthenticated) access on recent
builds: Windows requires SMB signing and guest sessions cannot be signed,
so the client refuses the guest session even with insecure guest logons
enabled. Use |--user| and |--pass| for Windows clients; Linux and macOS
clients can use guest.

If you do need guest access from a Windows client, run these as an
administrator and reboot (this lowers the client below its default
security, so prefer |--user|/|--pass|):

    Set-SmbClientConfiguration -EnableInsecureGuestLogons $true
    Set-SmbClientConfiguration -RequireSecuritySignature $false

### Caching

The |--vfs-cache-mode| flag controls how files are cached. For a cloud
remote, |--vfs-cache-mode full| improves random access and retries by
copying each accessed file into the cache directory. When the backend is a
**local** directory this only duplicates data, and because the cache size is
unlimited by default, accessing a very large file copies the whole thing
into the cache and can fill that disk, making the server unresponsive.

For serving a local drive, use |--vfs-cache-mode off| (or |minimal|). If you
do use a cache, bound it with |--vfs-cache-max-size| and
|--vfs-cache-min-free-space| (both off by default) -- note these drive a
background cleaner that evicts idle files, not a check that refuses a file
too big to fit.

### Connecting

The share is exposed with the name set by |--name| (default |rclone|),
so it is reachable as |\\HOST\rclone|.

Linux:

    smbclient //127.0.0.1/rclone -p 1445 -N
    sudo mount -t cifs //127.0.0.1/rclone /mnt -o port=1445,guest

macOS, in Finder use Go > Connect to Server:

    smb://127.0.0.1:1445/rclone

Windows 11 can connect to the non-standard port with:

    net use X: \\127.0.0.1\rclone /TCPPORT:1445

### VFS

Modifying files through the server requires VFS caching. Use the
|--vfs-cache-mode| flag to enable it (|--vfs-cache-mode full| is
recommended). Without a VFS cache the share is effectively read-only.

`, "|", "`") + strings.TrimSpace(vfs.Help()),
	Annotations: map[string]string{
		"versionIntroduced": "v1.75",
		"groups":            "Filter",
		"status":            "Experimental",
	},
	Run: Run,
}
