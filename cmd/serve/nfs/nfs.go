//go:build unix
// +build unix

// Package nfs implements a server to serve a VFS remote over NFSv3 protocol
//
// There is no authentication available on this server
// and it is served on loopback interface by default.
//
// This is primarily used for mounting a VFS remote
// in macOS, where FUSE-mounting mechanisms are usually not available.
package nfs

import (
	"context"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Options contains options for the NFS Server
type Options struct {
	ListenAddr string // Port to listen on
}

var opt Options

// AddFlags adds flags for the sftp
func AddFlags(flagSet *pflag.FlagSet, Opt *Options) {
	rc.AddOption("nfs", &Opt)
	flags.StringVarP(flagSet, &Opt.ListenAddr, "addr", "", Opt.ListenAddr, "IPaddress:Port or :Port to bind server to", "")
}

func init() {
	vfsflags.AddFlags(Command.Flags())
	AddFlags(Command.Flags(), &opt)
}

// Run the command
func Run(command *cobra.Command, args []string) {
	var f fs.Fs
	cmd.CheckArgs(1, 1, command, args)
	f = cmd.NewFsSrc(args)
	cmd.Run(false, true, command, func() error {
		s, err := NewServer(context.Background(), vfs.New(f, &vfsflags.Opt), &opt)
		if err != nil {
			return err
		}
		return s.Serve()
	})
}

// Command is the definition of the command
var Command = &cobra.Command{
	Use:   "nfs remote:path",
	Short: `Serve the remote as an NFS mount`,
	Long: `Create an NFS server that serves the given remote over the network.
	
The primary purpose for this command is to enable [mount command](/commands/rclone_mount/) on recent macOS versions where
installing FUSE is very cumbersome. 

Since this is running on NFSv3, no authentication method is available. Any client
will be able to access the data. To limit access, you can use serve NFS on loopback address
and rely on secure tunnels (such as SSH). For this reason, by default, a random TCP port is chosen and loopback interface is used for the listening address;
meaning that it is only available to the local machine. If you want other machines to access the
NFS mount over local network, you need to specify the listening address and port using ` + "`--addr`" + ` flag.

Modifying files through NFS protocol requires VFS caching. Usually you will need to specify ` + "`--vfs-cache-mode`" + `
in order to be able to write to the mountpoint (full is recommended). If you don't specify VFS cache mode,
the mount will be read-only.

To serve NFS over the network use following command:

    rclone serve nfs remote: --addr 0.0.0.0:$PORT --vfs-cache-mode=full

We specify a specific port that we can use in the mount command:

To mount the server under Linux/macOS, use the following command:
    
    mount -oport=$PORT,mountport=$PORT $HOSTNAME: path/to/mountpoint

Where ` + "`$PORT`" + ` is the same port number we used in the serve nfs command.

This feature is only available on Unix platforms.

` + vfs.Help,
	Annotations: map[string]string{
		"versionIntroduced": "v1.65",
		"groups":            "Filter",
		"status":            "Experimental",
	},
	Run: Run,
}
